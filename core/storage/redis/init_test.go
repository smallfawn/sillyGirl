package redis

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMasterStorageValuesRequestUsesPOST(t *testing.T) {
	originalMaster := master
	t.Cleanup(func() { master = originalMaster })

	method := ""
	path := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method = request.Method
		path = request.URL.Path
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":true}`))
	}))
	t.Cleanup(server.Close)

	master = server.URL
	request := newMasterStorageValuesRequest()
	if _, err := request.JSONBody(map[string]string{"sillyGirl.port": "8080"}); err != nil {
		t.Fatal(err)
	}
	if _, err := request.Bytes(); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost {
		t.Fatalf("master storage request method=%s; want POST", method)
	}
	if path != "/api/admin/storage/values" {
		t.Fatalf("master storage request path=%s", path)
	}
}
