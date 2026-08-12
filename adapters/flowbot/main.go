package flowbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	platform = "flowbot"

	defaultHost       = "127.0.0.1"
	defaultPort       = 7400
	defaultAPITime    = 15 * time.Second
	defaultMaxImage   = 20 << 20
	defaultPollWait   = 30 * time.Second
	readTimeout       = 45 * time.Second
	writeTimeout      = 5 * time.Second
	pingInterval      = 30 * time.Second
	dedupWindow       = 600 * time.Second
	reconnectBase     = 5 * time.Second
	reconnectMax      = 60 * time.Second
	reconnectAttempts = 5
)

var flowbot = core.MakeBucket(platform)

var runtime = struct {
	sync.Mutex
	cancel context.CancelFunc
}{}

var (
	compactNewlinePattern = regexp.MustCompile(`[\r\n]+`)
	cqCodePattern         = regexp.MustCompile(`\[CQ:[^\]]+\]`)
	cqImagePattern        = regexp.MustCompile(`(?i)\[CQ:image,([^\]]+)\]`)
)

// wsEnvelope is the top-level WebSocket frame pushed by FlowBot.
type wsEnvelope struct {
	Event string  `json:"event"`
	Data  *wsData `json:"data"`
}

// wsData is the message payload inside a "message" event.
type wsData struct {
	MessageID   string   `json:"message_id"`
	SessionID   string   `json:"session_id"`
	SessionType string   `json:"session_type"`
	SenderID    string   `json:"sender_id"`
	SenderName  string   `json:"sender_name"`
	SelfID      string   `json:"self_id"`
	GroupName   string   `json:"group_name"`
	Type        string   `json:"type"`
	Content     string   `json:"content"`
	ImageURL    string   `json:"image_url"`
	AtUsers     []string `json:"at_users"`
}

// sendRequest is the HTTP body posted to /api/v1/messages/send.
type sendRequest struct {
	SessionID   string   `json:"session_id"`
	Type        string   `json:"type"`
	Content     string   `json:"content,omitempty"`
	AtUsers     []string `json:"at_users,omitempty"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	ImageBase64 string   `json:"image_base64,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	ImagePath   string   `json:"image_path,omitempty"`
}

// sendResponse is the HTTP response from /api/v1/messages/send.
type sendResponse struct {
	Ret       int    `json:"ret"`
	MessageID string `json:"message_id"`
	Msg       string `json:"msg"`
	ErrMsg    string `json:"errmsg"`
}

type bot struct {
	host    string
	port    int
	apiKey  string
	tls     bool
	debug   bool
	botID   string
	adapter *core.Factory

	dedup   map[string]time.Time
	dedupMu sync.Mutex
}

func init() {
	for _, key := range []string{"token", "enable", "host", "port", "tls", "debug"} {
		key := key
		storage.Watch(flowbot, key, func(old, new, key string) *storage.Final {
			go restart()
			return nil
		})
	}
	go func() {
		time.Sleep(2 * time.Second)
		restart()
	}()
}

func restart() {
	runtime.Lock()
	if runtime.cancel != nil {
		runtime.cancel()
		runtime.cancel = nil
	}
	token := strings.TrimSpace(flowbot.GetString("token"))
	if token == "" {
		runtime.Unlock()
		core.Logs.Info("flowbot未启动：未配置 flowbot.token")
		return
	}
	if !enabled() {
		runtime.Unlock()
		core.Logs.Info("flowbot未启动：flowbot.enable=false")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.Unlock()

	go run(ctx, token)
}

func run(ctx context.Context, token string) {
	b := newBot(token)
	if err := b.start(ctx); err != nil && ctx.Err() == nil {
		core.Logs.Warn("flowbot启动失败：%v", err)
	}
}

func newBot(token string) *bot {
	port := flowbot.GetInt("port", defaultPort)
	if port <= 0 || port > 65535 {
		port = defaultPort
	}
	return &bot{
		host:   strings.TrimSpace(firstNonEmpty(flowbot.GetString("host"), defaultHost)),
		port:   port,
		apiKey: strings.TrimSpace(token),
		tls:    flowbot.GetBool("tls", false),
		debug:  flowbot.GetBool("debug", false),
		botID:  "default",
		dedup:  map[string]time.Time{},
	}
}

func (b *bot) start(ctx context.Context) error {
	b.adapter = &core.Factory{}
	b.adapter.Init(platform, b.botID, nil)
	defer b.adapter.Destroy()
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})
	core.Logs.Info("flowbot已初始化，准备连接 %s:%d", b.host, b.port)
	return b.serve(ctx)
}

func (b *bot) serve(ctx context.Context) error {
	attempts := 0
	delay := time.Duration(0)
	for {
		if ctx.Err() != nil {
			return nil
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}
		conn, err := b.dial(ctx)
		if err != nil {
			attempts++
			delay = backoff(attempts)
			core.Logs.Warn("flowbot连接失败(第%d次)：%v，%v后重连", attempts, err, delay)
			continue
		}
		core.Logs.Info("flowbot已连接 %s:%d", b.host, b.port)
		attempts = 0
		delay = 0
		b.readLoop(ctx, conn)
		if ctx.Err() != nil {
			return nil
		}
		core.Logs.Warn("flowbot连接断开，准备重连")
		delay = backoff(1)
	}
}

func (b *bot) dial(ctx context.Context) (*websocket.Conn, error) {
	u := url.URL{
		Scheme:   b.wsScheme(),
		Host:     fmt.Sprintf("%s:%d", b.host, b.port),
		Path:     "/api/v1/ws/messages",
		RawQuery: "token=" + url.QueryEscape(b.apiKey),
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, u.String(), http.Header{})
	return conn, err
}

func (b *bot) readLoop(ctx context.Context, conn *websocket.Conn) {
	conn.SetReadLimit(16 << 20)
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
					return
				}
			}
		}
	}()
	for {
		if ctx.Err() != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				core.Logs.Warn("flowbot读取失败：%v", err)
			}
			break
		}
		b.dispatch(data)
	}
	_ = conn.Close()
	<-done
}

func (b *bot) dispatch(data []byte) {
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		core.Logs.Warn("flowbot消息解析失败：%v", err)
		return
	}
	if env.Event != "message" || env.Data == nil {
		return
	}
	b.handleMessage(*env.Data)
}

func (b *bot) handleMessage(m wsData) {
	if !b.recordMessage(m.MessageID) {
		return
	}
	content := strings.TrimSpace(m.Content)
	if m.Type == "image" && content == "" {
		content = strings.TrimSpace(m.ImageURL)
	}
	if content == "" {
		return
	}
	userID := strings.TrimSpace(m.SenderID)
	if userID == "" {
		return
	}
	sessionID := strings.TrimSpace(m.SessionID)
	if sessionID == "" {
		return
	}
	chatID := ""
	if strings.EqualFold(m.SessionType, "group") {
		chatID = sessionID
		groupName := strings.TrimSpace(m.GroupName)
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    groupName,
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{b.botID},
		})
	}
	senderName := strings.TrimSpace(m.SenderName)
	core.CreateNickName(&core.Nickname{
		Value:    senderName,
		ID:       userID,
		Platform: platform,
		BotsID:   []string{b.botID},
	})
	params := map[string]interface{}{
		core.USER_ID:           userID,
		core.CHAT_ID:           chatID,
		core.CONETNT:           content,
		core.MESSAGE_ID:        m.MessageID,
		"user_name":            senderName,
		"chat_name":            strings.TrimSpace(m.GroupName),
		"flowbot_session_id":   sessionID,
		"flowbot_session_type": m.SessionType,
		"flowbot_image_url":    strings.TrimSpace(m.ImageURL),
		"flowbot_at_users":     m.AtUsers,
	}
	if b.debug {
		core.Logs.Debug("flowbot处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	sessionID := stringValue(msg["flowbot_session_id"])
	if sessionID == "" {
		sessionID = stringValue(msg[core.CHAT_ID])
	}
	if sessionID == "" {
		sessionID = stringValue(msg[core.USER_ID])
	}
	if sessionID == "" {
		core.Logs.Warn("flowbot发送失败：无法确定会话 session_id")
		return ""
	}
	atUsers := stringSliceValue(msg["flowbot_at_users"])
	lastID := ""
	for _, segment := range splitReplySegments(stringValue(msg[core.CONETNT])) {
		var (
			messageID string
			err       error
		)
		if segment.image {
			messageID, err = b.sendImage(ctx, sessionID, segment.value)
		} else {
			text := strings.TrimSpace(segment.value)
			if text == "" {
				continue
			}
			messageID, err = b.sendText(ctx, sessionID, text, atUsers)
		}
		if err != nil {
			core.Logs.Warn("flowbot发送失败：%v", err)
			continue
		}
		lastID = messageID
	}
	return lastID
}

func (b *bot) sendText(ctx context.Context, sessionID, text string, atUsers []string) (string, error) {
	req := sendRequest{
		SessionID: sessionID,
		Type:      "text",
		Content:   text,
		AtUsers:   atUsers,
	}
	return b.postMessage(ctx, req)
}

func (b *bot) sendImage(ctx context.Context, sessionID, source string) (string, error) {
	raw, err := b.readImageBytes(ctx, source)
	if err != nil {
		return "", fmt.Errorf("读取图片失败：%w", err)
	}
	if len(raw) > defaultMaxImage {
		return "", fmt.Errorf("图片超过 %d MB 限制", defaultMaxImage>>20)
	}
	req := sendRequest{
		SessionID:   sessionID,
		Type:        "image",
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
	}
	return b.postMessage(ctx, req)
}

func (b *bot) postMessage(ctx context.Context, req sendRequest) (string, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("%s://%s:%d/api/v1/messages/send", b.httpScheme(), b.host, b.port)
	reqCtx, cancel := context.WithTimeout(ctx, defaultAPITime)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out sendResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.Ret != 0 {
		return "", fmt.Errorf("ret=%d msg=%s errmsg=%s", out.Ret, out.Msg, out.ErrMsg)
	}
	return out.MessageID, nil
}

func (b *bot) wsScheme() string {
	if b.tls {
		return "wss"
	}
	return "ws"
}

func (b *bot) httpScheme() string {
	if b.tls {
		return "https"
	}
	return "http"
}

// recordMessage returns true when the message id is new (not seen within the
// dedupe window) and records it. Messages without an id are always processed.
func (b *bot) recordMessage(id string) bool {
	if strings.TrimSpace(id) == "" {
		return true
	}
	b.dedupMu.Lock()
	defer b.dedupMu.Unlock()
	if _, ok := b.dedup[id]; ok {
		return false
	}
	now := time.Now()
	b.dedup[id] = now
	if len(b.dedup) > 512 {
		for k, t := range b.dedup {
			if now.Sub(t) > dedupWindow {
				delete(b.dedup, k)
			}
		}
	}
	return true
}

func backoff(attempts int) time.Duration {
	if attempts <= 0 {
		return 0
	}
	d := reconnectBase * time.Duration(1<<uint(attempts-1))
	if d > reconnectMax {
		d = reconnectMax
	}
	return d
}

func enabled() bool {
	switch strings.ToLower(strings.TrimSpace(flowbot.GetString("enable"))) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func (b *bot) readImageBytes(ctx context.Context, source string) ([]byte, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("图片地址为空")
	}
	if strings.HasPrefix(strings.ToLower(source), "data:image/") {
		return decodeDataImage(source)
	}
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		reqCtx, cancel := context.WithTimeout(ctx, defaultAPITime)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if resp.ContentLength > defaultMaxImage {
			return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImage>>20)
		}
		return readLimited(resp.Body, defaultMaxImage)
	}
	path := source
	if err == nil && parsed.Scheme == "file" {
		path, err = url.PathUnescape(parsed.Path)
		if err != nil {
			return nil, err
		}
		if parsed.Host != "" {
			path = "//" + parsed.Host + path
		}
		if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		path = filepath.FromSlash(path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if info, statErr := file.Stat(); statErr == nil && info.Size() > defaultMaxImage {
		return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImage>>20)
	}
	return readLimited(file, defaultMaxImage)
}

func decodeDataImage(source string) ([]byte, error) {
	comma := strings.IndexByte(source, ',')
	if comma < 0 {
		return nil, fmt.Errorf("data URI 缺少数据段")
	}
	header, payload := strings.ToLower(source[:comma]), strings.TrimSpace(source[comma+1:])
	if !strings.Contains(header, ";base64") {
		decoded, err := url.PathUnescape(payload)
		if err != nil {
			return nil, err
		}
		if len(decoded) > defaultMaxImage {
			return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImage>>20)
		}
		return []byte(decoded), nil
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return nil, err
	}
	if len(data) > defaultMaxImage {
		return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImage>>20)
	}
	return data, nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("图片超过 %d MB 限制", limit>>20)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("图片内容为空")
	}
	return data, nil
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\\r", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = compactNewlinePattern.ReplaceAllString(value, "\n")
	return strings.TrimSpace(value)
}

type replySegment struct {
	image bool
	value string
}

func splitReplySegments(value string) []replySegment {
	matches := cqImagePattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return []replySegment{{value: value}}
	}
	segments := make([]replySegment, 0, len(matches)*2+1)
	position := 0
	for _, match := range matches {
		if match[0] > position {
			segments = append(segments, replySegment{value: value[position:match[0]]})
		}
		attrs := parseCQAttributes(value[match[2]:match[3]])
		source := firstNonEmpty(attrs["file"], attrs["url"])
		if source != "" {
			segments = append(segments, replySegment{image: true, value: source})
		}
		position = match[1]
	}
	if position < len(value) {
		segments = append(segments, replySegment{value: value[position:]})
	}
	return segments
}

func parseCQAttributes(value string) map[string]string {
	attrs := map[string]string{}
	lower := strings.ToLower(value)
	for _, key := range []string{"file", "url"} {
		marker := key + "=data:image/"
		if index := strings.Index(lower, marker); index >= 0 && (index == 0 || value[index-1] == ',') {
			attrs[key] = decodeCQValue(value[index+len(key)+1:])
			return attrs
		}
	}
	parts := strings.Split(value, ",")
	for _, part := range parts {
		key, raw, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		attrs[key] = decodeCQValue(strings.TrimSpace(raw))
	}
	return attrs
}

func decodeCQValue(value string) string {
	replacer := strings.NewReplacer(
		"&#44;", ",",
		"&#91;", "[",
		"&#93;", "]",
		"&amp;", "&",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func stringValue(value interface{}) string {
	return strings.TrimSpace(utils.Itoa(value))
}

func stringSliceValue(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, stringValue(item))
		}
		return out
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
