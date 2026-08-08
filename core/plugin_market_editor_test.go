package core

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
)

func TestSetMarketPluginStatusUpdatesAnnotation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, ".plugin-status-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "market-status.js")
	if err := os.WriteFile(path, []byte("// [title: Demo]\n// [status: true]\nconsole.log('ok');\n"), 0644); err != nil {
		t.Fatal(err)
	}
	previous := Functions
	f := &common.Function{UUID: "market-status-fixture", Title: "Demo", Type: NODE, Path: path}
	Functions = []*common.Function{f}
	t.Cleanup(func() { Functions = previous })

	requestStatus := func(status bool) map[string]interface{} {
		payload, _ := json.Marshal(gin.H{"status": status})
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/local-plugins/market-status-fixture/status", bytes.NewReader(payload))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "id", Value: "market-status-fixture"}}
		setMarketPluginStatus(ctx)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status update HTTP %d: %s", recorder.Code, recorder.Body.String())
		}
		response := map[string]interface{}{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		data, _ := response["data"].(map[string]interface{})
		return data
	}

	data := requestStatus(false)
	if data["status"] != false || pluginExecutionEnabled(f) {
		t.Fatalf("status response/state mismatch: data=%v status=%v", data, f.Status)
	}
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "// [status: false]") {
		t.Fatalf("status=false was not written to source:\n%s", script)
	}
	t.Logf("input: status=false; output: status=%v source=%q", *f.Status, "// [status: false]")

	data = requestStatus(true)
	if data["status"] != true || !pluginExecutionEnabled(f) {
		t.Fatalf("status response/state mismatch: data=%v status=%v", data, f.Status)
	}
	script, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(script), "[status:") != 1 || !strings.Contains(string(script), "// [status: true]") {
		t.Fatalf("status=true was not written cleanly:\n%s", script)
	}
	t.Logf("input: status=true; output: status=%v source=%q", *f.Status, "// [status: true]")
}

func TestSetMarketPluginStatusRejectsDependencyModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, ".plugin-module-status-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "shared-module.js")
	if err := os.WriteFile(path, []byte("// [module: true]\nmodule.exports = {};\n"), 0644); err != nil {
		t.Fatal(err)
	}
	previous := Functions
	Functions = []*common.Function{{UUID: "shared-module-fixture", Title: "Shared", Type: NODE, Module: true, Path: path}}
	t.Cleanup(func() { Functions = previous })

	payload, _ := json.Marshal(gin.H{"status": false})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/local-plugins/shared-module-fixture/status", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "shared-module-fixture"}}
	setMarketPluginStatus(ctx)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "依赖模块没有独立运行状态") {
		t.Fatalf("module status response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestValidateAndNormalizeLocalPluginContentAtStyle(t *testing.T) {
	content := `/**
 * @title Demo
 * @name demo
 * @desc demo desc
 * @version v1.0.0
 * @rule ^demo$
 * @public true
 */
console.log("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "* @public false") {
		t.Fatalf("public flag was not forced to false for @ metadata:\n%s", got)
	}
}

func TestValidateExistingPluginContentPreservesPublicCategory(t *testing.T) {
	content := `// [title: Public Demo]
// [name: publicDemo]
// [desc: public plugin]
// [version: v1.0.0]
// [rule: ^public$]
// [public: true]
`
	got, err := validateExistingPluginContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "// [public: true]") || strings.Contains(got, "// [public: false]") {
		t.Fatalf("editing an existing plugin changed its public category:\n%s", got)
	}
	created, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created, "// [public: false]") {
		t.Fatalf("new local plugin was not forced private:\n%s", created)
	}
}

func TestSaveMarketPluginScriptPreservesPublicCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	path := filepath.Join(nodePluginsRoot(), "publisher", "publicDemo.js")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := `// [title: Public Demo]
// [name: publicDemo]
// [desc: public plugin]
// [version: v1.0.0]
// [module: true]
// [public: true]
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	identity := "publisher/publicDemo"
	if err := AddNodePlugin(path, identity, NODE); err != nil {
		t.Fatal(err)
	}
	uuid := nameUuid(identity)
	t.Cleanup(func() {
		_ = os.Remove(path)
		_ = AddNodePlugin(path, identity, UNKNOWN)
	})

	edited := strings.Replace(original, "Public Demo", "Public Demo Edited", 1)
	payload, _ := json.Marshal(marketPluginScriptRequest{ID: uuid, Name: "publicDemo", Type: NODE, Content: edited})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/local-plugins/"+uuid, bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: uuid}}
	saveMarketPluginScript(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[public: true]") || strings.Contains(string(data), "[public: false]") {
		t.Fatalf("save changed public category:\n%s", data)
	}
	refreshed, err := nodeFunctionByID(uuid)
	if err != nil || refreshed.Title != "Public Demo Edited" || !refreshed.Public {
		t.Fatalf("reloaded plugin = (%#v, %v)", refreshed, err)
	}
}

func TestValidateAndNormalizeLocalPluginContentPythonAtStyle(t *testing.T) {
	content := `"""
@title PyDemo
@name pyDemo
@desc py desc
@version v1.0.0
@cron 12 8 * * *
"""
print("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "@public false") {
		t.Fatalf("public flag was not inserted for Python @ metadata:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentAcceptsBooleanActivatorAliases(t *testing.T) {
	content := `// [title: Startup]
// [name: startup]
// [desc: startup task]
// [version: v1.0.0]
// [on_start: YES]
`
	if _, err := validateAndNormalizeLocalPluginContent(content); err != nil {
		t.Fatalf("boolean activator alias was rejected: %v", err)
	}
}

func TestValidateAndNormalizeLocalPluginContentAcceptsModuleOnly(t *testing.T) {
	content := `// [title: Shared Tools]
// [name: sharedTools]
// [desc: shared module]
// [version: v1.0.0]
// [module: true]
`
	if _, err := validateAndNormalizeLocalPluginContent(content); err != nil {
		t.Fatalf("module-only plugin was rejected: %v", err)
	}
}

func TestWriteNewLocalMarketPluginRejectsExistingBaseName(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	target := filepath.Join(nodePluginsRoot(), "local")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "duplicate.py"), []byte("# existing"), 0644); err != nil {
		t.Fatal(err)
	}
	content := `// [title: Duplicate]
// [name: duplicate]
// [desc: duplicate plugin]
// [version: v1.0.0]
// [rule: ^duplicate$]
`
	if _, _, err := writeNewLocalMarketPlugin("duplicate", NODE, content); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("cross-runtime duplicate error = %v", err)
	}
}

func TestValidateAndNormalizeLocalPluginContentNewStyleNoSpace(t *testing.T) {
	content := `//[title: Demo]
//[name: demo]
//[desc: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
//[public: true]

console.log("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false]") {
		t.Fatalf("public flag was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentHashNewStyle(t *testing.T) {
	content := `##[title: PyDemo]
##[name: pyDemo]
##[desc: py desc]
##[version: v1.0.0]
##[cron: 12 8 * * *]

print("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "#[public: false]") {
		t.Fatalf("public flag was not inserted for hash metadata:\n%s", got)
	}
}

func TestForceLocalPluginPrivateBoolVariants(t *testing.T) {
	content := `//[title: Demo]
//[name: demo]
//[desc: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
//[public: YES]
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false]") {
		t.Fatalf("public YES was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentLegacyTrailingHelp(t *testing.T) {
	content := `//[title: Demo] 插件标题
//[name: demo] 文件名
//[desc: demo desc] 说明文字
//[version: v1.0.0] 版本说明
//[rule: ^demo$] 指令说明
//[public: YES] 是否公开
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false] 是否公开") {
		t.Fatalf("public YES with trailing help was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentRequiresName(t *testing.T) {
	content := `//[title: Demo]
//[desc: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
`
	if _, err := validateAndNormalizeLocalPluginContent(content); err == nil || !strings.Contains(err.Error(), "[name: 文件名]") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestValidateLocalPluginRequestNameMatchesMetaName(t *testing.T) {
	content := `//[title: Demo]
//[name: demoPlugin]
//[desc: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
`
	if err := validateLocalPluginRequestName("demoPlugin.js", content, NODE); err != nil {
		t.Fatalf("expected matching plugin name, got %v", err)
	}
	if err := validateLocalPluginRequestName("otherPlugin", content, NODE); err == nil || !strings.Contains(err.Error(), "必须和 [name: demoplugin] 一致") {
		t.Fatalf("expected name mismatch error, got %v", err)
	}
	if err := validateLocalPluginRequestName("", content, NODE); err == nil || !strings.Contains(err.Error(), "插件名称不能为空") {
		t.Fatalf("expected empty name error, got %v", err)
	}
}
