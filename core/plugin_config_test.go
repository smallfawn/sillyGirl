package core

import (
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

	if ctx.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected %d, got %d: %s", http.StatusNoContent, ctx.Writer.Status(), recorder.Body.String())
	}
}
