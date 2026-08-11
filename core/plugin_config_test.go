package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDeletePluginSettingsIsIdempotent(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "uuid", Value: "missing-plugin-config"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/?delete_schema=true", nil)

	deletePluginSettings(ctx)

	if ctx.Writer.Status() != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, ctx.Writer.Status(), recorder.Body.String())
	}
	envelope := map[string]interface{}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope) != 3 || envelope["status"] != true || envelope["message"] != "成功" || envelope["data"] != nil {
		t.Fatalf("unexpected response envelope: %#v", envelope)
	}
}
