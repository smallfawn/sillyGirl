package clawbot

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	platform               = "clawbot"
	defaultAPIBase         = "https://ilinkai.weixin.qq.com"
	defaultCDNBase         = "https://novac2c.cdn.weixin.qq.com/c2c"
	defaultChannelVer      = "2.4.6"
	defaultIlinkAppID      = "bot"
	defaultPollTimeout     = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second
	defaultMaxImageBytes   = 20 << 20
	messageTypeUser        = 1
	messageTypeBot         = 2
	messageItemText        = 1
	messageItemImage       = 2
	uploadMediaImage       = 1
	messageStateGenerating = 1
	messageStateFinish     = 2
)

var clawbot = core.MakeBucket(platform)

var runtime = struct {
	sync.Mutex
	cancel context.CancelFunc
}{}

var (
	compactNewlinePattern = regexp.MustCompile(`[\r\n]+`)
	cqCodePattern         = regexp.MustCompile(`\[CQ:[^\]]+\]`)
	cqImagePattern        = regexp.MustCompile(`(?i)\[CQ:image,([^\]]+)\]`)
)

type apiClient struct {
	baseURL        string
	cdnBaseURL     string
	token          string
	channelVersion string
	appID          string
	clientVersion  string
	client         *http.Client
	debug          bool
}

type baseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

type getUpdatesRequest struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      baseInfo `json:"base_info"`
}

type getUpdatesResponse struct {
	Ret                int             `json:"ret,omitempty"`
	ErrCode            int             `json:"errcode,omitempty"`
	ErrMsg             string          `json:"errmsg,omitempty"`
	Messages           []weixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf      string          `json:"get_updates_buf,omitempty"`
	LongPollingTimeout int             `json:"longpolling_timeout_ms,omitempty"`
}

type sendMessageRequest struct {
	Message  weixinMessage `json:"msg"`
	BaseInfo baseInfo      `json:"base_info"`
}

type sendMessageResponse struct {
	Ret     int    `json:"ret,omitempty"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type getUploadURLRequest struct {
	FileKey     string   `json:"filekey"`
	MediaType   int      `json:"media_type"`
	ToUserID    string   `json:"to_user_id"`
	RawSize     int      `json:"rawsize"`
	RawFileMD5  string   `json:"rawfilemd5"`
	FileSize    int      `json:"filesize"`
	NoNeedThumb bool     `json:"no_need_thumb"`
	AESKey      string   `json:"aeskey"`
	BaseInfo    baseInfo `json:"base_info"`
}

type getUploadURLResponse struct {
	Ret           int    `json:"ret,omitempty"`
	ErrCode       int    `json:"errcode,omitempty"`
	ErrMsg        string `json:"errmsg,omitempty"`
	UploadParam   string `json:"upload_param,omitempty"`
	UploadFullURL string `json:"upload_full_url,omitempty"`
}

type weixinMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64         `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64         `json:"delete_time_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []messageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
	RunID        string        `json:"run_id,omitempty"`
}

type messageItem struct {
	Type      int        `json:"type,omitempty"`
	MsgID     string     `json:"msg_id,omitempty"`
	TextItem  *textItem  `json:"text_item,omitempty"`
	ImageItem *imageItem `json:"image_item,omitempty"`
}

type textItem struct {
	Text string `json:"text,omitempty"`
}

type imageItem struct {
	Media   cdnMedia `json:"media"`
	MidSize int      `json:"mid_size"`
}

type cdnMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param"`
	AESKey            string `json:"aes_key"`
	EncryptType       int    `json:"encrypt_type"`
}

type replySegment struct {
	image bool
	value string
}

type bot struct {
	api      *apiClient
	adapter  *core.Factory
	botID    string
	syncBuf  string
	pollWait time.Duration
}

func init() {
	for _, key := range []string{"token", "enable", "api_base", "cdn_base_url", "debug", "channel_version"} {
		key := key
		storage.Watch(clawbot, key, func(old, new, key string) *storage.Final {
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
	token := strings.TrimSpace(clawbot.GetString("token"))
	if token == "" {
		runtime.Unlock()
		core.Logs.Info("clawbot未启动：未配置 clawbot.token")
		return
	}
	if !enabled() {
		runtime.Unlock()
		core.Logs.Info("clawbot未启动：clawbot.enable=false")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.Unlock()

	go run(ctx, token)
}

func run(ctx context.Context, token string) {
	b := &bot{
		api:      newAPIClient(token),
		botID:    "default",
		pollWait: defaultPollTimeout,
	}
	if err := b.start(ctx); err != nil && ctx.Err() == nil {
		core.Logs.Warn("clawbot启动失败：%v", err)
	}
}

func (b *bot) start(ctx context.Context) error {
	b.adapter = &core.Factory{}
	b.adapter.Init(platform, b.botID, nil)
	defer b.adapter.Destroy()
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})
	_ = b.api.notifyStart(ctx)
	core.Logs.Info("clawbot长轮询已启动")
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.api.notifyStop(stopCtx)
	}()
	return b.poll(ctx)
}

func (b *bot) poll(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		resp, err := b.api.getUpdates(ctx, b.syncBuf, b.pollWait)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			core.Logs.Warn("clawbot获取消息失败：%v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if resp.GetUpdatesBuf != "" {
			b.syncBuf = resp.GetUpdatesBuf
		}
		if resp.LongPollingTimeout > 0 {
			b.pollWait = time.Duration(resp.LongPollingTimeout)*time.Millisecond + 5*time.Second
		}
		if resp.Ret != 0 || resp.ErrCode != 0 {
			core.Logs.Warn("clawbot接口返回失败：ret=%d errcode=%d errmsg=%s", resp.Ret, resp.ErrCode, resp.ErrMsg)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, item := range resp.Messages {
			b.handleMessage(item)
		}
	}
}

func (b *bot) handleMessage(msg weixinMessage) {
	if msg.MessageType == messageTypeBot || msg.MessageState == messageStateGenerating {
		return
	}
	if msg.MessageType != 0 && msg.MessageType != messageTypeUser {
		return
	}
	content := normalizeText(messageText(msg))
	if content == "" {
		return
	}
	userID := strings.TrimSpace(msg.FromUserID)
	if userID == "" {
		userID = strings.TrimSpace(msg.ToUserID)
	}
	if userID == "" {
		return
	}
	chatID := strings.TrimSpace(msg.GroupID)
	if chatID != "" {
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    chatID,
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{b.botID},
		})
	}
	core.CreateNickName(&core.Nickname{
		Value:    userID,
		ID:       userID,
		Platform: platform,
		BotsID:   []string{b.botID},
	})
	params := map[string]interface{}{
		core.USER_ID:            userID,
		core.CHAT_ID:            core.ChatID(chatID),
		core.CONETNT:            content,
		core.MESSAGE_ID:         messageID(msg),
		"user_name":             userID,
		"chat_name":             chatID,
		"clawbot_context_token": msg.ContextToken,
		"clawbot_to_user_id":    userID,
		"clawbot_from_user_id":  msg.FromUserID,
		"clawbot_session_id":    msg.SessionID,
		"clawbot_run_id":        msg.RunID,
	}
	if b.api.debug {
		core.Logs.Debug("clawbot处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	toUserID := firstNonEmpty(
		stringValue(msg["clawbot_to_user_id"]),
		stringValue(msg[core.USER_ID]),
	)
	contextToken := stringValue(msg["clawbot_context_token"])
	if toUserID == "" || contextToken == "" {
		core.Logs.Warn("clawbot发送消息失败：缺少 to_user_id 或 context_token，ClawBot 仅支持在收到消息上下文内回复")
		return ""
	}
	runID := stringValue(msg["clawbot_run_id"])
	lastMessageID := ""
	for _, segment := range splitReplySegments(stringValue(msg[core.CONETNT])) {
		var (
			messageID string
			err       error
		)
		if segment.image {
			messageID, err = b.api.sendImage(ctx, toUserID, contextToken, runID, segment.value)
		} else {
			content := normalizeText(stripUnsupportedCQ(segment.value))
			if content == "" {
				continue
			}
			messageID, err = b.api.sendText(ctx, toUserID, contextToken, runID, content)
		}
		if err != nil {
			core.Logs.Warn("clawbot发送消息失败：%v", err)
			continue
		}
		lastMessageID = messageID
	}
	return lastMessageID
}

func newAPIClient(token string) *apiClient {
	return &apiClient{
		baseURL:        strings.TrimRight(firstNonEmpty(clawbot.GetString("api_base"), defaultAPIBase), "/"),
		cdnBaseURL:     strings.TrimRight(firstNonEmpty(clawbot.GetString("cdn_base_url"), defaultCDNBase), "/"),
		token:          strings.TrimSpace(token),
		channelVersion: firstNonEmpty(clawbot.GetString("channel_version"), defaultChannelVer),
		appID:          firstNonEmpty(clawbot.GetString("app_id"), defaultIlinkAppID),
		clientVersion:  firstNonEmpty(clawbot.GetString("client_version"), strconv.Itoa(buildClientVersion(defaultChannelVer))),
		client:         &http.Client{},
		debug:          clawbot.GetBool("debug", false),
	}
}

func (a *apiClient) getUpdates(ctx context.Context, syncBuf string, timeout time.Duration) (getUpdatesResponse, error) {
	var resp getUpdatesResponse
	err := a.post(ctx, "ilink/bot/getupdates", getUpdatesRequest{
		GetUpdatesBuf: syncBuf,
		BaseInfo:      a.baseInfo(),
	}, timeout, &resp)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return getUpdatesResponse{GetUpdatesBuf: syncBuf}, nil
		}
		return resp, err
	}
	return resp, nil
}

func (a *apiClient) sendText(ctx context.Context, toUserID string, contextToken string, runID string, text string) (string, error) {
	return a.sendItem(ctx, toUserID, contextToken, runID, messageItem{
		Type:     messageItemText,
		TextItem: &textItem{Text: text},
	})
}

func (a *apiClient) sendImage(ctx context.Context, toUserID string, contextToken string, runID string, source string) (string, error) {
	raw, err := a.readImage(ctx, source)
	if err != nil {
		return "", fmt.Errorf("读取图片失败：%w", err)
	}
	media, encryptedSize, err := a.uploadImage(ctx, toUserID, raw)
	if err != nil {
		return "", fmt.Errorf("上传图片失败：%w", err)
	}
	return a.sendItem(ctx, toUserID, contextToken, runID, messageItem{
		Type: messageItemImage,
		ImageItem: &imageItem{
			Media:   media,
			MidSize: encryptedSize,
		},
	})
}

func (a *apiClient) sendItem(ctx context.Context, toUserID string, contextToken string, runID string, item messageItem) (string, error) {
	clientID := fmt.Sprintf("sillygirl-clawbot-%d", time.Now().UnixNano())
	var resp sendMessageResponse
	err := a.post(ctx, "ilink/bot/sendmessage", sendMessageRequest{
		Message: weixinMessage{
			FromUserID:   "",
			ToUserID:     toUserID,
			ClientID:     clientID,
			MessageType:  messageTypeBot,
			MessageState: messageStateFinish,
			ContextToken: contextToken,
			RunID:        runID,
			ItemList:     []messageItem{item},
		},
		BaseInfo: a.baseInfo(),
	}, defaultAPITimeout, &resp)
	if err != nil {
		return "", err
	}
	if resp.Ret != 0 || resp.ErrCode != 0 {
		return "", fmt.Errorf("ret=%d errcode=%d errmsg=%s", resp.Ret, resp.ErrCode, resp.ErrMsg)
	}
	return clientID, nil
}

func (a *apiClient) uploadImage(ctx context.Context, toUserID string, raw []byte) (cdnMedia, int, error) {
	if len(raw) == 0 {
		return cdnMedia{}, 0, errors.New("图片内容为空")
	}
	if len(raw) > defaultMaxImageBytes {
		return cdnMedia{}, 0, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImageBytes>>20)
	}
	aesKey, err := randomBytes(aes.BlockSize)
	if err != nil {
		return cdnMedia{}, 0, err
	}
	fileKeyBytes, err := randomBytes(16)
	if err != nil {
		return cdnMedia{}, 0, err
	}
	fileKey := hex.EncodeToString(fileKeyBytes)
	sum := md5.Sum(raw)
	encryptedSize := aesEcbPaddedSize(len(raw))
	var uploadResp getUploadURLResponse
	err = a.post(ctx, "ilink/bot/getuploadurl", getUploadURLRequest{
		FileKey:     fileKey,
		MediaType:   uploadMediaImage,
		ToUserID:    toUserID,
		RawSize:     len(raw),
		RawFileMD5:  hex.EncodeToString(sum[:]),
		FileSize:    encryptedSize,
		NoNeedThumb: true,
		AESKey:      hex.EncodeToString(aesKey),
		BaseInfo:    a.baseInfo(),
	}, defaultAPITimeout, &uploadResp)
	if err != nil {
		return cdnMedia{}, 0, err
	}
	if uploadResp.Ret != 0 || uploadResp.ErrCode != 0 {
		return cdnMedia{}, 0, fmt.Errorf("ret=%d errcode=%d errmsg=%s", uploadResp.Ret, uploadResp.ErrCode, uploadResp.ErrMsg)
	}
	uploadURL := strings.TrimSpace(uploadResp.UploadFullURL)
	if uploadURL == "" {
		if strings.TrimSpace(uploadResp.UploadParam) == "" {
			return cdnMedia{}, 0, errors.New("getuploadurl 未返回 upload_param 或 upload_full_url")
		}
		uploadURL = a.cdnBaseURL + "/upload?encrypted_query_param=" + url.QueryEscape(uploadResp.UploadParam) + "&filekey=" + url.QueryEscape(fileKey)
	}
	encrypted, err := encryptAesEcb(raw, aesKey)
	if err != nil {
		return cdnMedia{}, 0, err
	}
	if len(encrypted) != encryptedSize {
		return cdnMedia{}, 0, fmt.Errorf("加密尺寸异常：want=%d got=%d", encryptedSize, len(encrypted))
	}
	encryptedParam, err := a.uploadCiphertext(ctx, uploadURL, encrypted)
	if err != nil {
		return cdnMedia{}, 0, err
	}
	return cdnMedia{
		EncryptQueryParam: encryptedParam,
		// The message protocol expects the lowercase hexadecimal AES key text
		// encoded as base64. Encoding the 16 raw key bytes makes the message
		// appear successful while WeChat renders the image as expired.
		AESKey:      base64.StdEncoding.EncodeToString([]byte(hex.EncodeToString(aesKey))),
		EncryptType: 1,
	}, encryptedSize, nil
}

func (a *apiClient) uploadCiphertext(ctx context.Context, uploadURL string, encrypted []byte) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, defaultAPITimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, uploadURL, bytes.NewReader(encrypted))
		if err != nil {
			cancel()
			return "", err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := a.client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if readErr != nil {
				err = readErr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				encryptedParam := strings.TrimSpace(resp.Header.Get("x-encrypted-param"))
				cancel()
				if encryptedParam == "" {
					return "", errors.New("CDN 上传成功但缺少 x-encrypted-param")
				}
				return encryptedParam, nil
			} else {
				err = fmt.Errorf("CDN HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
				if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					cancel()
					return "", err
				}
			}
		}
		cancel()
		lastErr = err
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
			}
		}
	}
	return "", lastErr
}

func (a *apiClient) notifyStart(ctx context.Context) error {
	var resp sendMessageResponse
	return a.post(ctx, "ilink/bot/msg/notifystart", map[string]interface{}{
		"base_info": a.baseInfo(),
	}, 10*time.Second, &resp)
}

func (a *apiClient) notifyStop(ctx context.Context) error {
	var resp sendMessageResponse
	return a.post(ctx, "ilink/bot/msg/notifystop", map[string]interface{}{
		"base_info": a.baseInfo(),
	}, 10*time.Second, &resp)
}

func (a *apiClient) post(ctx context.Context, endpoint string, body interface{}, timeout time.Duration, out interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	reqCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, a.baseURL+"/"+endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	for key, value := range a.headers() {
		req.Header.Set(key, value)
	}
	if a.debug {
		core.Logs.Debug("clawbot请求：POST %s bodyLen=%d", endpoint, len(payload))
	}
	httpResp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	data, err := utils.ReadAllLimit(httpResp.Body, 4<<20)
	if err != nil {
		return err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func (a *apiClient) headers() map[string]string {
	headers := map[string]string{
		"Content-Type":            "application/json",
		"AuthorizationType":       "ilink_bot_token",
		"Authorization":           "Bearer " + a.token,
		"X-WECHAT-UIN":            randomWechatUin(),
		"iLink-App-Id":            a.appID,
		"iLink-App-ClientVersion": a.clientVersion,
	}
	return headers
}

func (a *apiClient) baseInfo() baseInfo {
	return baseInfo{
		ChannelVersion: a.channelVersion,
	}
}

func enabled() bool {
	switch strings.ToLower(strings.TrimSpace(clawbot.GetString("enable"))) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func buildClientVersion(version string) int {
	parts := strings.Split(version, ".")
	values := []int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		values[i] = n
	}
	return ((values[0] & 0xff) << 16) | ((values[1] & 0xff) << 8) | (values[2] & 0xff)
}

func randomWechatUin() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	}
	value := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(value), 10)))
}

func messageText(msg weixinMessage) string {
	items := make([]string, 0, len(msg.ItemList))
	for _, item := range msg.ItemList {
		if item.Type == messageItemText && item.TextItem != nil && strings.TrimSpace(item.TextItem.Text) != "" {
			items = append(items, item.TextItem.Text)
		}
	}
	return strings.Join(items, "\n")
}

func messageID(msg weixinMessage) string {
	if msg.MessageID != 0 {
		return strconv.FormatInt(msg.MessageID, 10)
	}
	if msg.Seq != 0 {
		return strconv.FormatInt(msg.Seq, 10)
	}
	for _, item := range msg.ItemList {
		if strings.TrimSpace(item.MsgID) != "" {
			return strings.TrimSpace(item.MsgID)
		}
	}
	return ""
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\\r", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = compactNewlinePattern.ReplaceAllString(value, "\n")
	return strings.TrimSpace(value)
}

func stripUnsupportedCQ(value string) string {
	return strings.TrimSpace(cqCodePattern.ReplaceAllString(value, ""))
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

func (a *apiClient) readImage(ctx context.Context, source string) ([]byte, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("图片地址为空")
	}
	if strings.HasPrefix(strings.ToLower(source), "data:image/") {
		return decodeDataImage(source)
	}
	parsed, err := url.Parse(source)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		reqCtx, cancel := context.WithTimeout(ctx, defaultAPITimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if resp.ContentLength > defaultMaxImageBytes {
			return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImageBytes>>20)
		}
		return readLimited(resp.Body, defaultMaxImageBytes)
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
	if info, statErr := file.Stat(); statErr == nil && info.Size() > defaultMaxImageBytes {
		return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImageBytes>>20)
	}
	return readLimited(file, defaultMaxImageBytes)
}

func decodeDataImage(source string) ([]byte, error) {
	comma := strings.IndexByte(source, ',')
	if comma < 0 {
		return nil, errors.New("data URI 缺少数据段")
	}
	header, payload := strings.ToLower(source[:comma]), strings.TrimSpace(source[comma+1:])
	if !strings.Contains(header, ";base64") {
		decoded, err := url.PathUnescape(payload)
		if err != nil {
			return nil, err
		}
		if len(decoded) > defaultMaxImageBytes {
			return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImageBytes>>20)
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
	if len(data) > defaultMaxImageBytes {
		return nil, fmt.Errorf("图片超过 %d MB 限制", defaultMaxImageBytes>>20)
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
		return nil, errors.New("图片内容为空")
	}
	return data, nil
}

func randomBytes(size int) ([]byte, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

func aesEcbPaddedSize(size int) int {
	return ((size / aes.BlockSize) + 1) * aes.BlockSize
}

func encryptAesEcb(raw []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(raw)%aes.BlockSize
	padded := make([]byte, len(raw)+padding)
	copy(padded, raw)
	for i := len(raw); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	for offset := 0; offset < len(padded); offset += aes.BlockSize {
		block.Encrypt(padded[offset:offset+aes.BlockSize], padded[offset:offset+aes.BlockSize])
	}
	return padded, nil
}

func stringValue(value interface{}) string {
	return strings.TrimSpace(utils.Itoa(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
