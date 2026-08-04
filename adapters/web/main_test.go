package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core"
)

func TestWebUserEnqueueKeepsNewestWithoutBlocking(t *testing.T) {
	wu := &WebUser{Carry: make(chan WebMessage, 1)}
	wu.Enqueue(WebMessage{Content: "old"})
	done := make(chan struct{})
	go func() {
		wu.Enqueue(WebMessage{Content: "new"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("enqueue blocked on a full queue")
	}
	if got := (<-wu.Carry).Content; got != "new" {
		t.Fatalf("queued content = %q; want newest message", got)
	}
}

func TestWebChatRequestPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/web_chat", strings.NewReader(`{"rid":" admin-test_123 ","ctt":" listen "}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	rid, content, legacy, err := webChatRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rid != "admin-test_123" || content != "listen" || legacy {
		t.Fatalf("unexpected request: rid=%q content=%q legacy=%v", rid, content, legacy)
	}
}

func TestWebChatRequestRejectsEmptyPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/web_chat", strings.NewReader(`{"rid":"admin-test_123","ctt":"  "}`))
	if _, _, _, err := webChatRequest(ctx); err == nil {
		t.Fatal("empty POST message should be rejected")
	}
}

func TestAnonymousPollRejectedWhenPublicChatDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := core.MakeBucket("sillyGirl")
	old := settings.GetString("web_chat_public")
	if _, _, err := settings.Set("web_chat_public", false); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _, _ = settings.Set("web_chat_public", old) }()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/web_chat?rid=anonymous-probe_123", nil)
	started := time.Now()
	receiveWebChat(ctx)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("unauthorized poll was held open for %s", elapsed)
	}
}

func TestDrainWebMessages(t *testing.T) {
	carry := make(chan WebMessage, 3)
	carry <- WebMessage{Content: "one"}
	carry <- WebMessage{Content: "two"}
	rows := drainWebMessages(carry)
	if len(rows) != 2 || rows[0].Content != "one" || rows[1].Content != "two" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
