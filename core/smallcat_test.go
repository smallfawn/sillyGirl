package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSmallcatAccountUsagePrefersActualItems(t *testing.T) {
	value := decodeSmallcatJSONValue(json.RawMessage(`{
		"status": true,
		"data": {"count": 99, "limit": 30, "items": [{"openid":"a"},{"openid":"b"}]}
	}`))
	used, limit, ok := smallcatAccountUsage(value)
	if !ok || used != "2" || limit != "30" {
		t.Fatalf("smallcatAccountUsage = (%q, %q, %v); want (2, 30, true)", used, limit, ok)
	}
}

func TestSmallcatAccountUsageSupportsEncodedData(t *testing.T) {
	value := decodeSmallcatJSONValue(json.RawMessage(`{
		"status": true,
		"data": "{\"count\":3,\"limit\":10,\"items\":[{},{},{}]}"
	}`))
	used, limit, ok := smallcatAccountUsage(value)
	if !ok || used != "3" || limit != "10" {
		t.Fatalf("encoded smallcatAccountUsage = (%q, %q, %v)", used, limit, ok)
	}
}

func TestSmallcatAccountOpenIDs(t *testing.T) {
	value := decodeSmallcatJSONValue(json.RawMessage(`{
		"status":true,
		"data":{"items":[
			{"openid":"openid-a"},
			{"openId":"openid-b"},
			{"openid":"openid-a"},
			{"displayName":"missing"}
		]}
	}`))
	openids := smallcatAccountOpenIDs(value)
	if len(openids) != 2 || openids[0] != "openid-a" || openids[1] != "openid-b" {
		t.Fatalf("smallcatAccountOpenIDs = %#v", openids)
	}
}

func TestRedactSmallcatPanelsDoesNotMutateStoredSecret(t *testing.T) {
	panels := []SmallcatPanel{{ID: "panel-1", APIAuth: "secret-auth"}}
	redacted := redactSmallcatPanels(panels)
	if len(redacted) != 1 || redacted[0].APIAuth != "" {
		t.Fatalf("redacted panels = %#v", redacted)
	}
	if panels[0].APIAuth != "secret-auth" {
		t.Fatalf("source panel secret was mutated: %#v", panels[0])
	}
}

func TestRefreshSmallcatAccountUsageUsesAccountsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts" {
			t.Fatalf("request path = %q, want /api/accounts", r.URL.Path)
		}
		if got := r.Header.Get("auth"); got != "test-auth" {
			t.Fatalf("auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":{"count":0,"limit":30,"items":[{"openid":"a"},{"openid":"b"}]}}`))
	}))
	defer server.Close()

	panel := SmallcatPanel{Address: server.URL, APIAuth: "test-auth", AccountUsed: "0", AccountLimit: "1"}
	if err := refreshSmallcatAccountUsage(&panel); err != nil {
		t.Fatal(err)
	}
	if panel.AccountUsed != "2" || panel.AccountLimit != "30" {
		t.Fatalf("panel quota = %s/%s, want 2/30", panel.AccountUsed, panel.AccountLimit)
	}
}

func TestApplySmallcatAuthStatusDoesNotUseValidateQuotaCount(t *testing.T) {
	panel := SmallcatPanel{}
	applySmallcatAuthStatus(&panel, json.RawMessage(`{
		"status":true,
		"data":{"group":"vip","namespace":"10001","quota":{"count":0,"limit":30}}
	}`))
	if panel.AccountUsed != "" {
		t.Fatalf("validate quota count leaked into account usage: %q", panel.AccountUsed)
	}
	if panel.AccountLimit != "30" {
		t.Fatalf("account limit = %q, want 30", panel.AccountLimit)
	}
}
