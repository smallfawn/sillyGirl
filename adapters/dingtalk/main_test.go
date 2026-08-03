package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

type recordingReplier struct {
	webhook string
	content string
}

func (r *recordingReplier) SimpleReplyText(_ context.Context, webhook string, content []byte) error {
	r.webhook = webhook
	r.content = string(content)
	return nil
}

func TestReplyUsesMessageSessionAndStripsCQ(t *testing.T) {
	replier := &recordingReplier{}
	b := &bot{replier: replier}
	messageID := b.reply(context.Background(), map[string]interface{}{
		"content":                          "签到[CQ:image,url=https://example.com/a.png]成功",
		"message_id":                       "msg-1",
		"dingtalk_session_webhook":         "https://example.com/session",
		"dingtalk_session_webhook_expires": time.Now().Add(time.Minute).UnixMilli(),
	})
	if messageID != "msg-1" {
		t.Fatalf("message id = %q, want msg-1", messageID)
	}
	if replier.webhook != "https://example.com/session" || replier.content != "签到成功" {
		t.Fatalf("reply = webhook:%q content:%q", replier.webhook, replier.content)
	}
}

func TestReplyFallsBackToRememberedGroupSession(t *testing.T) {
	replier := &recordingReplier{}
	b := &bot{replier: replier}
	b.rememberSession("user-1", "chat-1", session{
		webhook:   "https://example.com/group-session",
		expiresAt: time.Now().Add(time.Minute).UnixMilli(),
	})
	b.reply(context.Background(), map[string]interface{}{
		"content": "ping",
		"user_id": "user-1",
		"chat_id": "chat-1",
	})
	if replier.webhook != "https://example.com/group-session" {
		t.Fatalf("webhook = %q", replier.webhook)
	}
}

func TestGroupSessionDoesNotBecomePrivatePushSession(t *testing.T) {
	b := &bot{}
	b.rememberSession("user-1", "chat-1", session{
		webhook:   "https://example.com/group-session",
		expiresAt: time.Now().Add(time.Minute).UnixMilli(),
	})
	if item := b.findSession("user-1", ""); item.webhook != "" {
		t.Fatalf("group webhook leaked into private push cache: %q", item.webhook)
	}
	if item := b.findSession("", "chat-1"); item.webhook != "https://example.com/group-session" {
		t.Fatalf("group session = %q", item.webhook)
	}
}

func TestHandlerWaitsForAdapterReadyOrShutdown(t *testing.T) {
	ready := make(chan struct{})
	done := make(chan struct{})
	b := &bot{ready: ready, done: done}
	returned := make(chan struct{})
	go func() {
		_, _ = b.handleMessage(context.Background(), &chatbot.BotCallbackDataModel{
			SenderStaffId: "user-1",
			Text:          chatbot.BotCallbackDataTextModel{Content: "ping"},
		})
		close(returned)
	}()
	select {
	case <-returned:
		t.Fatal("handler returned before adapter readiness")
	case <-time.After(20 * time.Millisecond):
	}
	close(done)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("handler remained blocked after shutdown")
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	replier := &recordingReplier{}
	b := &bot{replier: replier}
	b.reply(context.Background(), map[string]interface{}{
		"content":                          "ping",
		"dingtalk_session_webhook":         "https://example.com/expired",
		"dingtalk_session_webhook_expires": time.Now().Add(-time.Second).UnixMilli(),
	})
	if replier.webhook != "" {
		t.Fatalf("expired webhook was used: %q", replier.webhook)
	}
}

func TestWebhookReplierPayload(t *testing.T) {
	var payload struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	b := &bot{replier: &webhookReplier{client: server.Client()}}
	got := b.reply(context.Background(), map[string]interface{}{
		"content":                          "钉钉回复",
		"message_id":                       "msg-http",
		"dingtalk_session_webhook":         server.URL,
		"dingtalk_session_webhook_expires": time.Now().Add(time.Minute).UnixMilli(),
	})
	if got != "msg-http" || payload.MsgType != "text" || payload.Text.Content != "钉钉回复" {
		t.Fatalf("reply id=%q payload=%+v", got, payload)
	}
}

func TestWebhookReplierReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	replier := &webhookReplier{client: server.Client()}
	err := replier.SimpleReplyText(context.Background(), server.URL, []byte("hello"))
	if err == nil || err.Error() != "reply status 429: rate limited" {
		t.Fatalf("error = %v", err)
	}
}

func TestRetryOperationRecoversFromTransientFailure(t *testing.T) {
	attempts := 0
	retries := 0
	err := retryOperation(context.Background(), time.Millisecond, 2*time.Millisecond, func() error {
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

func TestRetryOperationStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := retryOperation(ctx, time.Hour, time.Hour, func() error {
		attempts++
		cancel()
		return fmt.Errorf("offline")
	}, nil)
	if err != context.Canceled || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
}

func TestSessionCleanupDeletesExpiredButKeepsReplacement(t *testing.T) {
	b := &bot{}
	old := session{webhook: "old", expiresAt: time.Now().Add(-time.Second).UnixMilli()}
	newer := session{webhook: "new", expiresAt: time.Now().Add(time.Minute).UnixMilli()}
	b.storeSession("user:expired", old)
	b.storeSession("user:replacement", old)
	b.storeSession("user:replacement", newer)
	b.deleteExpiredSessions(time.Now())
	if _, ok := b.sessions.Load("user:expired"); ok {
		t.Fatal("expired session remained cached")
	}
	value, ok := b.sessions.Load("user:replacement")
	if !ok || value.(session) != newer {
		t.Fatalf("replacement session was deleted: ok=%v value=%+v", ok, value)
	}
}
