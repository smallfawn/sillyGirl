package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNormalizeHTTPPort(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "plain", value: "8080", want: 8080},
		{name: "stored int", value: "d:8081", want: 8081},
		{name: "stored float", value: "f:8080.000000", want: 8080},
		{name: "invalid host", value: "f:8080.000000:bad", want: 8080},
		{name: "empty default", value: "", want: 8080},
		{name: "too high", value: "70000", want: 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeHTTPPort(tt.value); got != tt.want {
				t.Fatalf("normalizeHTTPPort(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestHTTPClientIPDoesNotTrustForwardedHeadersByDefault(t *testing.T) {
	t.Setenv("SILLYGIRL_TRUSTED_PROXIES", "none")
	engine := gin.New()
	if err := configureTrustedHTTPProxies(engine); err != nil {
		t.Fatal(err)
	}
	got := ""
	engine.GET("/", func(ctx *gin.Context) { got = ctx.ClientIP(); ctx.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:4321"
	req.Header.Set("X-Forwarded-For", "198.51.100.20")
	engine.ServeHTTP(httptest.NewRecorder(), req)
	if got != "203.0.113.10" {
		t.Fatalf("ClientIP trusted a spoofed forwarding header: %q", got)
	}
}

func TestCanonicalHTTPPortValue(t *testing.T) {
	for _, value := range []string{"8080", "d:8080", "f:8080.000000"} {
		port, stored := canonicalHTTPPortValue(value)
		if port != 8080 || stored != "8080" {
			t.Fatalf("canonicalHTTPPortValue(%q) = (%d, %q), want (8080, 8080)", value, port, stored)
		}
	}
}

func TestHTTPServerHasSlowlorisTimeouts(t *testing.T) {
	srv := httpServer(":8080")
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %v", srv.IdleTimeout)
	}
}

func TestServeAdminSPAUsesSuccessfulDocumentStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/containers/qinglong", nil)

	if !serveAdminSPA(ctx) {
		t.Fatal("serveAdminSPA did not serve embedded admin/index.html")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("admin SPA status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("admin SPA content type = %q", got)
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("admin SPA response body is empty")
	}
}
