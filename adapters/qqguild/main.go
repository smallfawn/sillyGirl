package qqguild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	qqmessage "github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/interaction/signature"
	"github.com/tencent-connect/botgo/interaction/webhook"
	"github.com/tencent-connect/botgo/openapi/options"
	"github.com/tencent-connect/botgo/token"
)

const (
	platform          = "qqguild"
	webhookPath       = "/qqguild/webhook"
	maxWebhookBody    = 1 << 20
	directSessionTTL  = 24 * time.Hour
	replySequenceTTL  = 5 * time.Minute
	cacheCleanupEvery = time.Minute
	tokenRetryInitial = 3 * time.Second
	tokenRetryMax     = time.Minute
)

var settings = core.MakeBucket(platform)

type messageAPI interface {
	PostMessage(context.Context, string, *dto.MessageToCreate, ...options.Option) (*dto.Message, error)
	PostDirectMessage(context.Context, *dto.DirectMessage, *dto.MessageToCreate, ...options.Option) (*dto.Message, error)
}

type bot struct {
	appID    string
	debug    bool
	api      messageAPI
	adapter  *core.Factory
	direct   sync.Map
	replySeq sync.Map
}

type directSession struct {
	guildID   string
	expiresAt int64
}

type replyCounter struct {
	sequence  atomic.Uint32
	expiresAt int64
}

var runtime = struct {
	sync.RWMutex
	generation  uint64
	cancel      context.CancelFunc
	credentials *token.QQBotCredentials
	bot         *bot
}{}

func init() {
	_ = event.RegisterHandlers(
		event.ATMessageEventHandler(handleATMessage),
		event.MessageEventHandler(handleGuildMessage),
		event.DirectMessageEventHandler(handleDirectMessage),
	)
	core.GinApi(core.POST, webhookPath, receiveWebhook)
	for _, key := range []string{"enable", "app_id", "app_secret", "sandbox", "debug"} {
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
	runtime.bot = nil
	if runtime.cancel != nil {
		runtime.cancel()
		runtime.cancel = nil
	}
	runtime.generation++
	generation := runtime.generation
	runtime.credentials = nil
	appID := strings.TrimSpace(settings.GetString("app_id"))
	appSecret := strings.TrimSpace(settings.GetString("app_secret"))
	if !core.AdapterConfigEnabled(platform) {
		runtime.Unlock()
		core.Logs.Info("qqguild机器人未启动：qqguild.enable=false")
		return
	}
	if appID == "" || appSecret == "" {
		runtime.Unlock()
		core.Logs.Info("qqguild机器人未启动：未配置 qqguild.app_id/app_secret")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.credentials = &token.QQBotCredentials{AppID: appID, AppSecret: appSecret}
	runtime.Unlock()

	go run(ctx, generation, appID, appSecret)
}

func run(ctx context.Context, generation uint64, appID, appSecret string) {
	credentials := &token.QQBotCredentials{AppID: appID, AppSecret: appSecret}
	tokenSource := token.NewQQBotTokenSource(credentials)
	// OpenAPI obtains a fresh token from tokenSource before every request. An
	// explicit first fetch validates credentials without starting BotGo's
	// background refresher, whose repeated-failure path panics in v0.2.1.
	if err := retryToken(ctx, tokenRetryInitial, tokenRetryMax, func() error {
		_, err := tokenSource.Token()
		return err
	}, func(err error) {
		core.Logs.Warn("qqguild获取 access token 失败，稍后重试：%v", err)
	}); err != nil {
		return
	}
	if ctx.Err() != nil {
		return
	}

	var api messageAPI
	if settings.GetBool("sandbox", false) {
		api = botgo.NewSandboxOpenAPI(appID, tokenSource).WithTimeout(10 * time.Second).SetDebug(settings.GetBool("debug", false))
	} else {
		api = botgo.NewOpenAPI(appID, tokenSource).WithTimeout(10 * time.Second).SetDebug(settings.GetBool("debug", false))
	}
	b := &bot{
		appID:   appID,
		debug:   settings.GetBool("debug", false),
		api:     api,
		adapter: &core.Factory{},
	}
	b.adapter.Init(platform, appID, nil)
	b.adapter.SetReplyHandler(func(msg map[string]interface{}) string {
		return b.reply(ctx, msg)
	})
	go b.cleanupCaches(ctx)

	runtime.Lock()
	if runtime.generation != generation || ctx.Err() != nil {
		runtime.Unlock()
		b.adapter.Destroy()
		return
	}
	runtime.bot = b
	runtime.Unlock()
	core.Logs.Info("qqguild机器人(%s) Webhook 已就绪：%s", appID, webhookPath)

	<-ctx.Done()
	runtime.Lock()
	if runtime.bot == b {
		runtime.bot = nil
	}
	runtime.Unlock()
	b.adapter.Destroy()
}

func receiveWebhook(c *gin.Context) {
	credentials := currentCredentials()
	if credentials == nil || !core.AdapterConfigEnabled(platform) {
		c.String(http.StatusServiceUnavailable, "qqguild adapter is not ready")
		return
	}
	serveWebhook(c.Writer, c.Request, credentials)
}

func serveWebhook(w http.ResponseWriter, request *http.Request, credentials *token.QQBotCredentials) {
	if credentials == nil || strings.TrimSpace(credentials.AppSecret) == "" {
		http.Error(w, "qqguild credentials are not ready", http.StatusServiceUnavailable)
		return
	}
	defer request.Body.Close()
	reader := http.MaxBytesReader(w, request.Body, maxWebhookBody)
	body, err := io.ReadAll(reader)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "webhook body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read webhook body failed", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty webhook body", http.StatusBadRequest)
		return
	}
	pass, err := signature.Verify(credentials.AppSecret, request.Header, body)
	if err != nil || !pass {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}
	payload := &dto.WSPayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		http.Error(w, "invalid webhook payload", http.StatusBadRequest)
		return
	}
	payload.RawMessage = body
	payload.Session = &dto.Session{AppID: credentials.AppID}
	switch payload.OPCode {
	case dto.HTTPCallbackValidation:
		data, ok := payload.Data.(map[string]interface{})
		if !ok {
			http.Error(w, "invalid validation data", http.StatusBadRequest)
			return
		}
		plainToken, plainOK := data["plain_token"].(string)
		eventTS, timeOK := data["event_ts"].(string)
		if !plainOK || !timeOK || plainToken == "" || eventTS == "" {
			http.Error(w, "invalid validation data", http.StatusBadRequest)
			return
		}
		response := webhook.GenValidationACK(&dto.WHValidationReq{PlainToken: plainToken, EventTs: eventTS}, request.Header, credentials.AppSecret)
		if len(response) == 0 {
			http.Error(w, "generate validation response failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(response)
	case dto.WSHeartbeat:
		sequence, ok := payload.Data.(float64)
		if !ok || sequence < 0 || sequence > math.MaxUint32 || math.Trunc(sequence) != sequence {
			http.Error(w, "invalid heartbeat sequence", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, webhook.GenHeartbeatACK(uint32(sequence)))
	case dto.WSDispatchEvent:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, webhook.GenDispatchACK(event.ParseAndHandle(payload) == nil))
	default:
		http.Error(w, "unsupported webhook opcode", http.StatusBadRequest)
	}
}

func currentBot() *bot {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.bot
}

func currentCredentials() *token.QQBotCredentials {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.credentials
}

func handleATMessage(_ *dto.WSPayload, data *dto.WSATMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, false)
}

func handleGuildMessage(_ *dto.WSPayload, data *dto.WSMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, false)
}

func handleDirectMessage(_ *dto.WSPayload, data *dto.WSDirectMessageData) error {
	if data == nil {
		return nil
	}
	message := dto.Message(*data)
	return dispatchMessage(&message, true)
}

func dispatchMessage(message *dto.Message, direct bool) error {
	b := currentBot()
	if b == nil {
		return fmt.Errorf("qqguild adapter is not ready")
	}
	return b.receive(message, direct)
}

func (b *bot) receive(message *dto.Message, direct bool) error {
	if message == nil || message.Author == nil || message.Author.Bot {
		return nil
	}
	content := strings.TrimSpace(qqmessage.ETLInput(message.Content))
	userID := strings.TrimSpace(message.Author.ID)
	if content == "" || userID == "" {
		return nil
	}

	chatID := ""
	chatName := ""
	if direct {
		dmGuildID := firstNonEmpty(message.GuildID, message.SrcGuildID)
		if dmGuildID != "" {
			b.rememberDirect(userID, dmGuildID)
		}
	} else {
		chatID = strings.TrimSpace(message.ChannelID)
		chatName = chatID
	}
	userName := firstNonEmpty(memberNick(message.Member), message.Author.Username, userID)
	core.CreateNickName(&core.Nickname{
		Value:    userName,
		ID:       userID,
		Platform: platform,
		BotsID:   []string{b.appID},
	})
	if chatID != "" {
		core.CreateNickName(&core.Nickname{
			Group:    true,
			Value:    chatName,
			ID:       chatID,
			Platform: platform,
			BotsID:   []string{b.appID},
		})
	}

	params := map[string]interface{}{
		core.USER_ID:              userID,
		core.CHAT_ID:              chatID,
		core.CONETNT:              content,
		core.MESSAGE_ID:           message.ID,
		"user_name":               userName,
		"chat_name":               chatName,
		"qqguild_guild_id":        message.GuildID,
		"qqguild_source_guild_id": message.SrcGuildID,
		"qqguild_channel_id":      message.ChannelID,
		"qqguild_direct":          direct,
	}
	if b.debug {
		core.Logs.Debug("qqguild处理消息：%s", string(utils.JsonMarshal(params)))
	}
	b.adapter.Receive(params)
	return nil
}

func (b *bot) reply(ctx context.Context, msg map[string]interface{}) string {
	content := strings.TrimSpace(stripUnsupportedCQ(stringValue(msg[core.CONETNT])))
	if content == "" {
		return ""
	}
	messageID := strings.TrimSpace(stringValue(msg[core.MESSAGE_ID]))
	payload := &dto.MessageToCreate{
		Content: content,
		MsgID:   messageID,
		MsgSeq:  b.nextReplySeq(messageID),
	}
	requestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	var (
		result *dto.Message
		err    error
	)
	if boolValue(msg["qqguild_direct"]) {
		userID := strings.TrimSpace(stringValue(msg[core.USER_ID]))
		dmGuildID := firstNonEmpty(
			stringValue(msg["qqguild_guild_id"]),
			stringValue(msg["qqguild_source_guild_id"]),
			b.directGuildID(userID),
		)
		if dmGuildID == "" {
			core.Logs.Warn("qqguild发送私信失败：缺少私信 guild_id")
			return ""
		}
		result, err = b.api.PostDirectMessage(requestCtx, &dto.DirectMessage{GuildID: dmGuildID}, payload)
	} else {
		channelID := firstNonEmpty(stringValue(msg[core.CHAT_ID]), stringValue(msg["qqguild_channel_id"]))
		if channelID == "" {
			core.Logs.Warn("qqguild发送频道消息失败：缺少 channel_id")
			return ""
		}
		result, err = b.api.PostMessage(requestCtx, channelID, payload)
	}
	if err != nil {
		core.Logs.Warn("qqguild发送消息失败：%v", err)
		return ""
	}
	if result == nil {
		return ""
	}
	return result.ID
}

func (b *bot) directGuildID(userID string) string {
	if value, ok := b.direct.Load(userID); ok {
		item := value.(directSession)
		if time.Now().UnixMilli() < item.expiresAt {
			return item.guildID
		}
		b.direct.CompareAndDelete(userID, item)
	}
	return ""
}

func (b *bot) rememberDirect(userID, guildID string) {
	item := directSession{
		guildID:   guildID,
		expiresAt: time.Now().Add(directSessionTTL).UnixMilli(),
	}
	b.direct.Store(userID, item)
}

func (b *bot) nextReplySeq(messageID string) uint32 {
	if messageID == "" {
		return 0
	}
	for {
		now := time.Now()
		counter := &replyCounter{expiresAt: now.Add(replySequenceTTL).UnixMilli()}
		value, loaded := b.replySeq.LoadOrStore(messageID, counter)
		if !loaded {
			return counter.sequence.Add(1)
		}
		current, ok := value.(*replyCounter)
		if ok && now.UnixMilli() < current.expiresAt {
			return current.sequence.Add(1)
		}
		b.replySeq.CompareAndDelete(messageID, value)
	}
}

func (b *bot) cleanupCaches(ctx context.Context) {
	ticker := time.NewTicker(cacheCleanupEvery)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			b.deleteExpiredCaches(now)
		case <-ctx.Done():
			return
		}
	}
}

func (b *bot) deleteExpiredCaches(now time.Time) {
	nowMillis := now.UnixMilli()
	b.direct.Range(func(key, value interface{}) bool {
		item, ok := value.(directSession)
		if !ok || nowMillis >= item.expiresAt {
			b.direct.CompareAndDelete(key, value)
		}
		return true
	})
	b.replySeq.Range(func(key, value interface{}) bool {
		counter, ok := value.(*replyCounter)
		if !ok || nowMillis >= counter.expiresAt {
			b.replySeq.CompareAndDelete(key, value)
		}
		return true
	})
}

func retryToken(ctx context.Context, initialDelay, maxDelay time.Duration, operation func() error, onRetry func(error)) error {
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

func memberNick(member *dto.Member) string {
	if member == nil {
		return ""
	}
	return member.Nick
}

func boolValue(value interface{}) bool {
	switch value := value.(type) {
	case bool:
		return value
	default:
		switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
		case "true", "1", "yes", "on":
			return true
		default:
			return false
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
