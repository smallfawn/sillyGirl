package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDaidaiPanelTokenValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/open-api/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"token","token_type":"Bearer","expires_in":86400}}`))
	}))
	defer server.Close()

	panel, err := testDaidaiPanel(DaidaiPanel{
		Address:   server.URL,
		AppKey:    "app-key",
		AppSecret: "app-secret",
	})
	if err != nil {
		t.Fatalf("testDaidaiPanel returned error: %v", err)
	}
	if panel.Status != "online" {
		t.Fatalf("unexpected panel status: %s", panel.Status)
	}
}
