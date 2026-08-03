package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	platform           = "dingtalk"
	streamRetryInitial = 3 * time.Second
	streamRetryMax     = time.Minute
)

var settings = core.MakeBucket(platform)

var runtime = struct {
	sync.Mutex
	cancel context.CancelFunc
}{}

type textReplier interface {
	SimpleReplyText(context.Context, string, []byte) error
}

type webhookReplier struct {
	client *http.Client
}

type session struct {
	webhook   string
	expiresAt int64
}

type bot struct {
	clientID string
	debug    bool
	adapter  *core.Factory
	replier  textReplier
	sessions sync.Map
	ready    chan struct{}
	done     <-chan struct{}
}

func init() {
	for _, key := range []string{"enable", "client_id", "client_secret", "debug"} {
		key := key
		storage.Watch(settings, key, func(old, new, key string) *storage.Final {
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
	clientID := strings.TrimSpace(settings.GetString("client_id"))
	clientSecret := strings.TrimSpace(settings.GetString("client_secret"))
	if !core.AdapterConfigEnabled(platform) {
		runtime.Unlock()
		core.Logs.Info("dingtalk机器人未启动：dingtalk.enable=false")
		return
	}
	if clientID == "" || clientSecret == "" {
		runtime.Unlock()
		core.Logs.Info("dingtalk机器人未启动：未配置 dingtalk.client_id/client_secret")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.Unlock()

	go run(ctx, clientID, clientSecret)
}

func run(ctx context.Context, clientID, clientSecret string) {
	b := &bot{
		clientID: clientID,
		debug:    settings.GetBool("debug", false),
		replier: &webhookReplier{client: &http.Client{
			Timeout: 10 * time.Second,
		}},
		ready: make(chan struct{}),
		done:  ctx.Done(),
	}
	stream := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig(clientID, clientSecret)),
	)
	defer stream.Close()
	stream.RegisterChatBotCallbackRouter(b.handleMessage)
	if err := retryOperation(ctx, streamRetryInitial, streamRetryMax, func() error {
		return stream.Start(ctx)
	}, func(err error) {
		core.Logs.Warn("dingtalk机器人连接失败，稍后重试：%v", err)
	}); err != nil {
		close(b.ready)
		return
	}
	if ctx.Err() != nil {
		return
	}

	b.adapter = &core.Factory{}
	b.adapter.Init(platform, clientID, nil)
	defer b.adapter.Destroy()
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})
	go b.cleanupSessions(ctx)
	close(b.ready)
	core.Logs.Info("dingtalk机器人(%s) Stream 已连接", clientID)
	<-ctx.Done()
}

func (b *bot) handleMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	if data == nil {
		return nil, nil
	}
	if b.ready != nil {
		select {
		case <-b.ready:
		case <-b.done:
			return nil, nil
		case <-ctx.Done():
			return nil, nil
		}
	}
	if b.adapter == nil {
		return nil, nil
	}
	content := strings.TrimSpace(data.Text.Content)
	userID := firstNonEmpty(data.SenderStaffId, data.SenderId)
	if content == "" || userID == "" || userID == data.ChatbotUserId {
		return nil, nil
	}

	chatID := ""
	if data.ConversationType != "1" {
		chatID = strings.TrimSpace(data.ConversationId)
	}
	item := session{webhook: strings.TrimSpace(data.SessionWebhook), expiresAt: data.SessionWebhookExpiredTime}
	b.rememberSession(userID, chatID, item)

	core.CreateNickName(&core.Nickname{
		Value:    firstNonEmpty(data.SenderNick, userID),
		ID:       userID,
		Platform: platform,
		BotsID:   []string{b.clientID},
	})
	if chatID != "" {
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    firstNonEmpty(data.ConversationTitle, chatID),
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{b.clientID},
		})
	}

	params := map[string]interface{}{
		core.USER_ID:                       userID,
		core.CHAT_ID:                       chatID,
		core.CONETNT:                       content,
		core.MESSAGE_ID:                    data.MsgId,
		"user_name":                        firstNonEmpty(data.SenderNick, userID),
		"chat_name":                        firstNonEmpty(data.ConversationTitle, chatID),
		"dingtalk_conversation_id":         data.ConversationId,
		"dingtalk_conversation_type":       data.ConversationType,
		"dingtalk_session_webhook":         item.webhook,
		"dingtalk_session_webhook_expires": item.expiresAt,
		"dingtalk_robot_code":              data.RobotCode,
	}
	if b.debug {
		core.Logs.Debug("dingtalk处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
	return nil, nil
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	content := strings.TrimSpace(stripUnsupportedCQ(stringValue(msg[core.CONETNT])))
	if content == "" {
		return ""
	}
	item := session{
		webhook:   strings.TrimSpace(stringValue(msg["dingtalk_session_webhook"])),
		expiresAt: int64Value(msg["dingtalk_session_webhook_expires"]),
	}
	if !item.valid(time.Now()) {
		item = b.findSession(stringValue(msg[core.USER_ID]), stringValue(msg[core.CHAT_ID]))
	}
	if !item.valid(time.Now()) {
		core.Logs.Warn("dingtalk发送消息失败：当前会话的 sessionWebhook 缺失或已过期")
		return ""
	}
	requestCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := b.replier.SimpleReplyText(requestCtx, item.webhook, []byte(content)); err != nil {
		core.Logs.Warn("dingtalk发送消息失败：%v", err)
		return ""
	}
	return stringValue(msg[core.MESSAGE_ID])
}

func (b *bot) rememberSession(userID, chatID string, item session) {
	if !item.valid(time.Now()) {
		return
	}
	if chatID != "" {
		b.storeSession("chat:"+chatID, item)
		return
	}
	if userID != "" {
		b.storeSession("user:"+userID, item)
	}
}

func (b *bot) storeSession(key string, item session) {
	b.sessions.Store(key, item)
}

func (b *bot) cleanupSessions(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			b.deleteExpiredSessions(now)
		case <-ctx.Done():
			return
		}
	}
}

func (b *bot) deleteExpiredSessions(now time.Time) {
	b.sessions.Range(func(key, value interface{}) bool {
		item, ok := value.(session)
		if !ok || !item.valid(now) {
			b.sessions.CompareAndDelete(key, value)
		}
		return true
	})
}

func (b *bot) findSession(userID, chatID string) session {
	for _, key := range []string{"chat:" + chatID, "user:" + userID} {
		if strings.HasSuffix(key, ":") {
			continue
		}
		if value, ok := b.sessions.Load(key); ok {
			item := value.(session)
			if item.valid(time.Now()) {
				return item
			}
			b.sessions.Delete(key)
		}
	}
	return session{}
}

func (s session) valid(now time.Time) bool {
	if strings.TrimSpace(s.webhook) == "" {
		return false
	}
	return s.expiresAt == 0 || now.UnixMilli() < s.expiresAt
}

func (r *webhookReplier) SimpleReplyText(ctx context.Context, webhook string, content []byte) error {
	payload, err := json.Marshal(struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{Content: string(content)},
	})
	if err != nil {
		return fmt.Errorf("encode reply: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create reply request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	client := r.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send reply: %w", err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if readErr != nil {
		return fmt.Errorf("read reply response: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("reply status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func retryOperation(ctx context.Context, initialDelay, maxDelay time.Duration, operation func() error, onRetry func(error)) error {
	delay := initialDelay
	for {
		err := operation()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if onRetry != nil {
			onRetry(err)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

func stripUnsupportedCQ(value string) string {
	for {
		start := strings.Index(value, "[CQ:")
		if start < 0 {
			return strings.TrimSpace(value)
		}
		end := strings.Index(value[start:], "]")
		if end < 0 {
			return strings.TrimSpace(value[:start])
		}
		value = value[:start] + value[start+end+1:]
	}
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func int64Value(value interface{}) int64 {
	switch value := value.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		var result int64
		_, _ = fmt.Sscan(stringValue(value), &result)
		return result
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
