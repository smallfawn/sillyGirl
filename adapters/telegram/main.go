package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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

var telegram = core.MakeBucket("telegram")
var runtime = struct {
	sync.Mutex
	cancel context.CancelFunc
}{}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      T      `json:"result"`
}

type user struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      *user  `json:"from"`
	Chat      chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
}

type update struct {
	UpdateID      int64    `json:"update_id"`
	Message       *message `json:"message"`
	EditedMessage *message `json:"edited_message"`
}

type bot struct {
	token   string
	baseURL string
	client  *http.Client
	adapter *core.Factory
	self    user
	debug   bool
}

type replySegment struct {
	kind  string
	value string
}

const (
	replySegmentText  = "text"
	replySegmentImage = "image"
)

var cqImagePattern = regexp.MustCompile(`\[CQ:image,([^\]]+)\]`)

func init() {
	storage.Watch(telegram, "token", func(old, new, key string) *storage.Final {
		go restart()
		return nil
	})
	storage.Watch(telegram, "enable", func(old, new, key string) *storage.Final {
		go restart()
		return nil
	})
	storage.Watch(telegram, "api_base", func(old, new, key string) *storage.Final {
		go restart()
		return nil
	})
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
	token := getToken()
	if token == "" {
		runtime.Unlock()
		core.Logs.Info("telegram机器人未启动：未配置 telegram.token")
		return
	}
	if !enabled() {
		runtime.Unlock()
		core.Logs.Info("telegram机器人未启动：telegram.enable=false")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.Unlock()

	go run(ctx, token)
}

func run(ctx context.Context, token string) {
	b := &bot{
		token:   token,
		baseURL: strings.TrimRight(firstNonEmpty(telegram.GetString("api_base"), "https://api.telegram.org"), "/"),
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
		debug: telegram.GetBool("debug", false),
	}
	if err := b.start(ctx); err != nil && ctx.Err() == nil {
		core.Logs.Warn("telegram机器人启动失败：%v", err)
	}
}

func (b *bot) start(ctx context.Context) error {
	me, err := b.getMe(ctx)
	if err != nil {
		return err
	}
	b.self = me
	b.adapter = &core.Factory{}
	b.adapter.Init("telegram", strconv.FormatInt(me.ID, 10), nil)
	defer b.adapter.Destroy()
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})

	_ = b.deleteWebhook(ctx)
	core.Logs.Info("telegram机器人(%s)轮询已启动", botName(me))
	return b.poll(ctx)
}

func (b *bot) poll(ctx context.Context) error {
	offset := int64(0)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			core.Logs.Warn("telegram获取消息失败：%v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, item := range updates {
			if item.UpdateID >= offset {
				offset = item.UpdateID + 1
			}
			b.handleUpdate(item)
		}
	}
}

func (b *bot) handleUpdate(item update) {
	msg := item.Message
	if msg == nil {
		msg = item.EditedMessage
	}
	if msg == nil || msg.From == nil || msg.From.IsBot || msg.From.ID == b.self.ID {
		return
	}
	content := strings.TrimSpace(firstNonEmpty(msg.Text, msg.Caption))
	if content == "" {
		return
	}
	userID := strconv.FormatInt(msg.From.ID, 10)
	chatID := ""
	if msg.Chat.Type != "private" {
		chatID = strconv.FormatInt(msg.Chat.ID, 10)
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    chatName(msg.Chat),
			ID:       chatID,
			Platform: "telegram",
			BotsID:   []string{strconv.FormatInt(b.self.ID, 10)},
		})
	}
	core.CreateNickName(&core.Nickname{
		Value:    userName(*msg.From),
		ID:       userID,
		Platform: "telegram",
		BotsID:   []string{strconv.FormatInt(b.self.ID, 10)},
	})
	params := map[string]interface{}{
		core.USER_ID:       userID,
		core.CHAT_ID:       chatID,
		core.CONETNT:       content,
		core.MESSAGE_ID:    strconv.FormatInt(msg.MessageID, 10),
		"user_name":        userName(*msg.From),
		"chat_name":        chatName(msg.Chat),
		"telegram_chat_id": strconv.FormatInt(msg.Chat.ID, 10),
	}
	if b.debug {
		core.Logs.Debug("telegram处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	chatID := stringValue(msg[core.CHAT_ID])
	if chatID == "" {
		chatID = stringValue(msg["telegram_chat_id"])
	}
	if chatID == "" {
		chatID = stringValue(msg[core.USER_ID])
	}
	segments := splitReplySegments(stringValue(msg[core.CONETNT]))
	if chatID == "" || len(segments) == 0 {
		return ""
	}

	lastMessageID := ""
	for _, segment := range segments {
		var (
			messageID string
			err       error
		)
		switch segment.kind {
		case replySegmentImage:
			messageID, err = b.sendPhoto(ctx, chatID, segment.value)
		default:
			messageID, err = b.sendText(ctx, chatID, segment.value)
		}
		if err != nil {
			core.Logs.Warn("telegram发送消息失败：%v", err)
			return ""
		}
		if messageID != "" {
			lastMessageID = messageID
		}
	}
	return lastMessageID
}

func (b *bot) sendText(ctx context.Context, chatID string, text string) (string, error) {
	var resp apiResponse[message]
	err := b.post(ctx, "sendMessage", map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}, &resp)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(resp.Result.MessageID, 10), nil
}

func (b *bot) sendPhoto(ctx context.Context, chatID string, photo string) (string, error) {
	if data, filename, ok := decodeDataImage(photo); ok {
		return b.uploadPhoto(ctx, chatID, filename, bytes.NewReader(data))
	}
	if isExistingFile(photo) {
		file, err := os.Open(photo)
		if err != nil {
			return "", err
		}
		defer file.Close()
		return b.uploadPhoto(ctx, chatID, filepath.Base(photo), file)
	}
	var resp apiResponse[message]
	err := b.post(ctx, "sendPhoto", map[string]interface{}{
		"chat_id": chatID,
		"photo":   photo,
	}, &resp)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(resp.Result.MessageID, 10), nil
}

func (b *bot) uploadPhoto(ctx context.Context, chatID string, filename string, reader io.Reader) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", chatID); err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("photo", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, reader); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiURL("sendPhoto"), &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var resp apiResponse[message]
	if err := b.do(req, &resp); err != nil {
		return "", err
	}
	return strconv.FormatInt(resp.Result.MessageID, 10), nil
}

func (b *bot) getMe(ctx context.Context) (user, error) {
	var resp apiResponse[user]
	if err := b.get(ctx, "getMe", nil, &resp); err != nil {
		return user{}, err
	}
	return resp.Result, nil
}

func (b *bot) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	params := map[string]string{
		"timeout": "30",
	}
	if offset > 0 {
		params["offset"] = strconv.FormatInt(offset, 10)
	}
	var resp apiResponse[[]update]
	if err := b.get(ctx, "getUpdates", params, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (b *bot) deleteWebhook(ctx context.Context) error {
	var resp apiResponse[bool]
	return b.post(ctx, "deleteWebhook", map[string]interface{}{}, &resp)
}

func (b *bot) get(ctx context.Context, method string, params map[string]string, out interface{}) error {
	url := b.apiURL(method)
	if len(params) > 0 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		q := req.URL.Query()
		for key, value := range params {
			q.Set(key, value)
		}
		req.URL.RawQuery = q.Encode()
		return b.do(req, out)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return b.do(req, out)
}

func (b *bot) post(ctx context.Context, method string, payload map[string]interface{}, out interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiURL(method), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return b.do(req, out)
}

func (b *bot) do(req *http.Request, out interface{}) error {
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := utils.ReadAllLimit(resp.Body, 4<<20)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	var base apiResponse[json.RawMessage]
	if err := json.Unmarshal(data, &base); err == nil && !base.OK {
		return errors.New(firstNonEmpty(base.Description, "telegram api 返回失败"))
	}
	return nil
}

func (b *bot) apiURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", b.baseURL, b.token, method)
}

func getToken() string {
	return strings.TrimSpace(telegram.GetString("token"))
}

func enabled() bool {
	value := strings.TrimSpace(strings.ToLower(telegram.GetString("enable")))
	return value != "false" && value != "0" && value != "no" && value != "off"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func userName(u user) string {
	name := strings.TrimSpace(strings.Join([]string{u.FirstName, u.LastName}, " "))
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return strconv.FormatInt(u.ID, 10)
}

func chatName(c chat) string {
	return firstNonEmpty(c.Title, c.Username, strings.TrimSpace(strings.Join([]string{c.FirstName, c.LastName}, " ")), strconv.FormatInt(c.ID, 10))
}

func botName(u user) string {
	return firstNonEmpty(u.Username, userName(u), strconv.FormatInt(u.ID, 10))
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func splitReplySegments(text string) []replySegment {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	segments := make([]replySegment, 0, 2)
	last := 0
	matches := cqImagePattern.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		if match[0] > last {
			appendReplyTextSegment(&segments, text[last:match[0]])
		}
		attrs := parseCQParams(text[match[2]:match[3]])
		image := firstNonEmpty(attrs["file"], attrs["url"])
		if image != "" {
			segments = append(segments, replySegment{kind: replySegmentImage, value: image})
		}
		last = match[1]
	}
	if last < len(text) {
		appendReplyTextSegment(&segments, text[last:])
	}
	return segments
}

func appendReplyTextSegment(segments *[]replySegment, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	*segments = append(*segments, replySegment{kind: replySegmentText, value: text})
}

func parseCQParams(raw string) map[string]string {
	params := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		params[key] = decodeCQValue(value)
	}
	return params
}

func decodeCQValue(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(
		"&#44;", ",",
		"&#91;", "[",
		"&#93;", "]",
		"&amp;", "&",
	)
	return replacer.Replace(value)
}

func decodeDataImage(value string) ([]byte, string, bool) {
	if !strings.HasPrefix(value, "data:image/") {
		return nil, "", false
	}
	header, encoded, ok := strings.Cut(value, ",")
	if !ok || !strings.Contains(header, ";base64") {
		return nil, "", false
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", false
	}
	ext := "png"
	if mimeType, _, ok := strings.Cut(strings.TrimPrefix(header, "data:"), ";"); ok {
		switch mimeType {
		case "image/jpeg":
			ext = "jpg"
		case "image/gif":
			ext = "gif"
		case "image/webp":
			ext = "webp"
		}
	}
	return data, "image." + ext, true
}

func isExistingFile(value string) bool {
	if value == "" || strings.Contains(value, "://") {
		return false
	}
	info, err := os.Stat(value)
	return err == nil && !info.IsDir()
}
