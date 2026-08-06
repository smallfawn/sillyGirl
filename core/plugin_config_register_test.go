package core

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
const { plugin } = require("sillygirl");
new plugin.Form({
  token: plugin.Form.string().title("Token").default("abc")
});
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

func TestRegisterPythonPluginConfigSchema(t *testing.T) {
	if _, _, err := resolvePythonCommand(); err != nil {
		t.Skipf("python 3.12 not available: %v", err)
	}

	uuid := "test_python_plugin_config_register"
	pluginConfigSchemas.Set(uuid, "")
	t.Cleanup(func() {
		pluginConfigSchemas.Set(uuid, "")
	})

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "config-plugin.py")
	script := `
import sillygirl_config_schema_dependency_that_does_not_exist as optional_dependency
optional_dependency.initialize().configure()

from sillygirl import plugin

plugin.Form({
    "token": plugin.Form.string().title("Token").default("abc")
})
`
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	if err := registerPythonPluginConfigSchema(scriptPath, uuid); err != nil {
		t.Fatalf("registerPythonPluginConfigSchema failed: %v", err)
	}
	got := pluginConfigSchemas.GetString(uuid)
	if !strings.Contains(got, `"token"`) || !strings.Contains(got, `"Token"`) {
		t.Fatalf("schema was not stored correctly: %s", got)
	}
}

func TestPythonConfigPreloadStubsMissingDependencies(t *testing.T) {
	bin, args := anyPythonCommandForTest(t)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatal(err)
	}
	preload, err := ensurePythonConfigPreload(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}

	pluginPath := filepath.Join(dir, "missing-dependency-plugin.py")
	plugin := `
import sillygirl_dependency_that_is_definitely_not_installed as optional_dependency
from sillygirl_dependency_that_is_definitely_not_installed.deep import Client
optional_dependency.initialize().configure(Client)

from sillygirl import plugin

plugin.Form({
    "token": plugin.Form.string().title("Token")
})

raise RuntimeError("business code ran after config export")
`
	if err := os.WriteFile(pluginPath, []byte(plugin), 0644); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(dir, "schema.json")
	output, err := runPythonConfigPreloadForTest(bin, args, preload, pluginPath, runtimeDir, schemaPath)
	if err != nil {
		t.Fatalf("config preload failed: %v: %s", err, output)
	}
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	schema := map[string]interface{}{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("invalid schema JSON: %v: %s", err, data)
	}
	pluginSchema, _ := schema["plugin"].(map[string]interface{})
	properties, _ := pluginSchema["properties"].(map[string]interface{})
	token, _ := properties["token"].(map[string]interface{})
	if token["title"] != "Token" {
		t.Fatalf("unexpected schema: %s", data)
	}
}

func TestPythonConfigPreloadSkipsBusinessCode(t *testing.T) {
	bin, args := anyPythonCommandForTest(t)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		t.Fatal(err)
	}
	preload, err := ensurePythonConfigPreload(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(dir, "broken-plugin.py")
	source := `
from sillygirl import plugin
DEFAULT_TOKEN = "from-constant"
plugin.Form({"token": plugin.Form.string().title("Token").default(DEFAULT_TOKEN)})
raise RuntimeError("business code must not run while exporting forms")
`
	if err := os.WriteFile(pluginPath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := runPythonConfigPreloadForTest(
		bin, args, preload, pluginPath, runtimeDir, filepath.Join(dir, "schema.json"),
	)
	if err != nil {
		t.Fatalf("config preload ran business code: %v: %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "schema.json"))
	if err != nil || !strings.Contains(string(data), `"token"`) || !strings.Contains(string(data), `"from-constant"`) {
		t.Fatalf("plugin form was not exported: %v: %s", err, data)
	}
}

func anyPythonCommandForTest(t *testing.T) (string, []string) {
	t.Helper()
	candidates := [][]string{{"python3"}, {"python"}, {"py", "-3"}}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err != nil {
			continue
		}
		if pythonCommandVersion(candidate[0], candidate[1:]) != "" {
			return candidate[0], candidate[1:]
		}
	}
	t.Skip("python is not available")
	return "", nil
}

func runPythonConfigPreloadForTest(bin string, args []string, preload, plugin, pythonPath, schemaPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmdArgs := append(append([]string{}, args...), "-u", preload, plugin)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+pythonPath,
		"SILLYGIRL_CONFIG_REGISTER_ONLY=true",
		"SILLYGIRL_CONFIG_SCHEMA_FILE="+schemaPath,
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	return string(output), err
}
