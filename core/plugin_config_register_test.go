package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterNodePluginConfigSchema(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}

	uuid := "test_plugin_config_register"
	pluginConfigSchemas.Set(uuid, "")
	t.Cleanup(func() {
		pluginConfigSchemas.Set(uuid, "")
	})

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "config-plugin.js")
	script := `
const { sillyGirlCreateSchema, SillyGirlPluginConfig } = require("sillygirl");
new SillyGirlPluginConfig(sillyGirlCreateSchema.object({
  token: sillyGirlCreateSchema.string().setTitle("Token").setDefault("abc")
}));
`
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := registerNodePluginConfigSchema(scriptPath, uuid); err != nil {
		t.Fatalf("registerNodePluginConfigSchema failed: %v", err)
	}
	got := pluginConfigSchemas.GetString(uuid)
	if !strings.Contains(got, `"token"`) || !strings.Contains(got, `"Token"`) {
		t.Fatalf("schema was not stored correctly: %s", got)
	}
}
