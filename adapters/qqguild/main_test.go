package qqguild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tencent-connect/botgo/dto"
	qqmessage "github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/interaction/signature"
	"github.com/tencent-connect/botgo/openapi/options"
	"github.com/tencent-connect/botgo/token"
)

type recordingAPI struct {
	channelID string
	dmGuildID string
	groupID   string
	c2cUserID string
	payload   *dto.MessageToCreate
}

func (api *recordingAPI) PostMessage(_ context.Context, channelID string, msg *dto.MessageToCreate, _ ...options.Option) (*dto.Message, error) {
	api.channelID = channelID
	api.payload = msg
	return &dto.Message{ID: "reply-channel"}, nil
}

func (api *recordingAPI) PostDirectMessage(_ context.Context, dm *dto.DirectMessage, msg *dto.MessageToCreate, _ ...options.Option) (*dto.Message, error) {
	api.dmGuildID = dm.GuildID
	api.payload = msg
	return &dto.Message{ID: "reply-direct"}, nil
}

func (api *recordingAPI) PostGroupMessage(_ context.Context, groupID string, msg dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	api.groupID = groupID
	api.payload = msg.(*dto.MessageToCreate)
	return &dto.Message{ID: "reply-group"}, nil
}

func (api *recordingAPI) PostC2CMessage(_ context.Context, userID string, msg dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	api.c2cUserID = userID
	api.payload = msg.(*dto.MessageToCreate)
	return &dto.Message{ID: "reply-c2c"}, nil
}

func TestChannelReplyUsesChannelAndSourceMessage(t *testing.T) {
	api := &recordingAPI{}
	b := &bot{api: api}
	got := b.reply(context.Background(), map[string]interface{}{
		"content":            "pong[CQ:image,url=https://example.com/a.png]",
		"message_id":         "source-1",
		"chat_id":            "channel-1",
		"qqguild_channel_id": "channel-fallback",
	})
	if got != "reply-channel" {
		t.Fatalf("reply id = %q", got)
	}
	if api.channelID != "channel-1" || api.payload.Content != "pong" || api.payload.MsgID != "source-1" || api.payload.MsgSeq != 1 {
		t.Fatalf("channel reply = channel:%q payload:%+v", api.channelID, api.payload)
	}
}

func TestDirectReplyUsesRememberedDMGuild(t *testing.T) {
	api := &recordingAPI{}
	b := &bot{api: api}
	b.rememberDirect("user-1", "dm-guild-1")
	got := b.reply(context.Background(), map[string]interface{}{
		"content":        "pong",
		"message_id":     "source-2",
		"user_id":        "user-1",
		"qqguild_direct": true,
	})
	if got != "reply-direct" || api.dmGuildID != "dm-guild-1" {
		t.Fatalf("direct reply id=%q guild=%q", got, api.dmGuildID)
	}
}

func TestGroupReplyUsesOfficialGroupAPI(t *testing.T) {
	api := &recordingAPI{}
	b := &bot{api: api}
	got := b.reply(context.Background(), map[string]interface{}{
		"content":          "pong",
		"message_id":       "source-group",
		"chat_id":          "group-fallback",
		"qqguild_group_id": "group-1",
		"qqguild_scene":    sceneGroup,
	})
	if got != "reply-group" || api.groupID != "group-1" || api.payload.MsgID != "source-group" {
		t.Fatalf("group reply id=%q group=%q payload=%+v", got, api.groupID, api.payload)
	}
}

func TestC2CReplyUsesOfficialUserAPI(t *testing.T) {
	api := &recordingAPI{}
	b := &bot{api: api}
	got := b.reply(context.Background(), map[string]interface{}{
		"content":       "pong",
		"message_id":    "source-c2c",
		"user_id":       "openid-1",
		"qqguild_scene": sceneC2C,
	})
	if got != "reply-c2c" || api.c2cUserID != "openid-1" || api.payload.MsgID != "source-c2c" {
		t.Fatalf("c2c reply id=%q user=%q payload=%+v", got, api.c2cUserID, api.payload)
	}
}

func TestWebsocketIntentsIncludeOfficialQQMessages(t *testing.T) {
	t.Logf("websocket intents=%d group_messages_bit=%d", websocketIntents, dto.IntentGroupMessages)
	if websocketIntents&dto.IntentGroupMessages == 0 {
		t.Fatalf("websocket intents %d omit GROUP_AT/C2C messages", websocketIntents)
	}
}

func TestConfigRestartIsDebouncedUntilAfterWrites(t *testing.T) {
	called := make(chan struct{}, 2)
	debouncer := &restartDebouncer{
		delay: 10 * time.Millisecond,
		action: func() {
			called <- struct{}{}
		},
	}
	for range 6 {
		debouncer.schedule()
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("debounced restart did not run")
	}
	select {
	case <-called:
		t.Fatal("multiple config writes caused multiple restarts")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestBotGoLogsRedactCredentialsAndTokens(t *testing.T) {
	const input = `req:{"clientSecret":"secret-value"} access:{"access_token":"access-value"} Authorization: QQBot token-value`
	got := sanitizeBotGoLog(input)
	for _, leaked := range []string{"secret-value", "access-value", "token-value"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("sensitive value %q leaked in %q", leaked, got)
		}
	}
	if count := strings.Count(got, "<redacted>"); count != 3 {
		t.Fatalf("redaction count=%d log=%q", count, got)
	}
}

func TestReplySequenceIncrementsPerSourceMessage(t *testing.T) {
	b := &bot{}
	if got := b.nextReplySeq("source"); got != 1 {
		t.Fatalf("first seq = %d", got)
	}
	if got := b.nextReplySeq("source"); got != 2 {
		t.Fatalf("second seq = %d", got)
	}
	if got := b.nextReplySeq(""); got != 0 {
		t.Fatalf("proactive seq = %d", got)
	}
	b.replySeq.Store("expired", &replyCounter{
		expiresAt: time.Now().Add(-time.Second).UnixMilli(),
	})
	if got := b.nextReplySeq("expired"); got != 1 {
		t.Fatalf("expired sequence did not reset: %d", got)
	}
}

func TestQQMentionIsRemoved(t *testing.T) {
	if got := stringsTrimQQMention("<@!123456> 解析 口令"); got != "解析 口令" {
		t.Fatalf("content = %q", got)
	}
}

func stringsTrimQQMention(content string) string {
	return firstNonEmpty(qqmessage.ETLInput(content))
}

func TestOfficialWebhookValidationRoundTrip(t *testing.T) {
	const (
		secret     = "qqguild-test-secret"
		plainToken = "plain-token-123"
		timestamp  = "1722672000"
	)
	body := []byte(`{"op":13,"d":{"plain_token":"plain-token-123","event_ts":"1722672000"}}`)
	request := httptest.NewRequest("POST", webhookPath, bytes.NewReader(body))
	request.ContentLength = -1
	request.Header.Set(signature.HeaderTimestamp, timestamp)
	sig, err := signature.Generate(secret, request.Header, body)
	if err != nil {
		t.Fatalf("generate signature: %v", err)
	}
	request.Header.Set(signature.HeaderSig, sig)
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: secret})

	var response struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode validation response %q: %v", recorder.Body.String(), err)
	}
	if response.PlainToken != plainToken || response.Signature == "" {
		t.Fatalf("validation response = %+v", response)
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	body := []byte(`{"op":13,"d":{"plain_token":"x","event_ts":"1"}}`)
	request := httptest.NewRequest(http.MethodPost, webhookPath, bytes.NewReader(body))
	request.Header.Set(signature.HeaderTimestamp, "1")
	request.Header.Set(signature.HeaderSig, "00")
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: "secret"})
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestWebhookBodyLimit(t *testing.T) {
	body := bytes.Repeat([]byte("x"), maxWebhookBody+1)
	request := httptest.NewRequest(http.MethodPost, webhookPath, bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: "secret"})
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestWebhookRejectsMissingCredentials(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, webhookPath, bytes.NewReader([]byte(`{}`)))
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestWebhookRejectsSignedInvalidJSON(t *testing.T) {
	request := signedWebhookRequest(t, "secret", []byte(`not-json`))
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: "secret"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestWebhookRejectsMalformedHeartbeat(t *testing.T) {
	request := signedWebhookRequest(t, "secret", []byte(`{"op":1,"d":"bad"}`))
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: "secret"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestWebhookHeartbeatRoundTrip(t *testing.T) {
	request := signedWebhookRequest(t, "secret", []byte(`{"op":1,"d":7}`))
	recorder := httptest.NewRecorder()
	serveWebhook(recorder, request, &token.QQBotCredentials{AppID: "app-1", AppSecret: "secret"})
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"op":11,"d":7}` {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRetryTokenRecoversFromTransientFailure(t *testing.T) {
	attempts := 0
	retries := 0
	err := retryToken(context.Background(), time.Millisecond, 2*time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("temporary failure %d", attempts)
		}
		return nil
	}, func(error) {
		retries++
	})
	if err != nil || attempts != 3 || retries != 2 {
		t.Fatalf("err=%v attempts=%d retries=%d", err, attempts, retries)
	}
}

func TestRetryTokenStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := retryToken(ctx, time.Hour, time.Hour, func() error {
		attempts++
		cancel()
		return fmt.Errorf("offline")
	}, nil)
	if err != context.Canceled || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestExpiredDirectSessionIsRemoved(t *testing.T) {
	b := &bot{}
	b.direct.Store("user-1", directSession{
		guildID:   "expired-guild",
		expiresAt: time.Now().Add(-time.Second).UnixMilli(),
	})
	if guildID := b.directGuildID("user-1"); guildID != "" {
		t.Fatalf("expired guild id = %q", guildID)
	}
	if _, ok := b.direct.Load("user-1"); ok {
		t.Fatal("expired direct session remained cached")
	}
}

func TestCacheCleanupDeletesExpiredEntries(t *testing.T) {
	b := &bot{}
	b.direct.Store("expired-user", directSession{guildID: "guild", expiresAt: time.Now().Add(-time.Second).UnixMilli()})
	b.direct.Store("active-user", directSession{guildID: "guild", expiresAt: time.Now().Add(time.Minute).UnixMilli()})
	b.replySeq.Store("expired-message", &replyCounter{expiresAt: time.Now().Add(-time.Second).UnixMilli()})
	b.replySeq.Store("active-message", &replyCounter{expiresAt: time.Now().Add(time.Minute).UnixMilli()})
	b.deleteExpiredCaches(time.Now())

	if _, ok := b.direct.Load("expired-user"); ok {
		t.Fatal("expired direct session remained cached")
	}
	if _, ok := b.replySeq.Load("expired-message"); ok {
		t.Fatal("expired reply counter remained cached")
	}
	if _, ok := b.direct.Load("active-user"); !ok {
		t.Fatal("active direct session was deleted")
	}
	if _, ok := b.replySeq.Load("active-message"); !ok {
		t.Fatal("active reply counter was deleted")
	}
}

func TestNormalizeConnectionMode(t *testing.T) {
	tests := map[string]string{
		"":          modeWebhook,
		"webhook":   modeWebhook,
		"invalid":   modeWebhook,
		"websocket": modeWebsocket,
		" WS ":      modeWebsocket,
		"wss":       modeWebsocket,
	}
	for input, want := range tests {
		if got := normalizeConnectionMode(input); got != want {
			t.Fatalf("normalizeConnectionMode(%q) = %q; want %q", input, got, want)
		}
	}
}

func signedWebhookRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, webhookPath, bytes.NewReader(body))
	request.ContentLength = -1
	request.Header.Set(signature.HeaderTimestamp, "1722672000")
	sig, err := signature.Generate(secret, request.Header, body)
	if err != nil {
		t.Fatalf("generate signature: %v", err)
	}
	request.Header.Set(signature.HeaderSig, sig)
	return request
}
