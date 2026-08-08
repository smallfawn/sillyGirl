package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
)

func TestPluginMarketFlagsAreSerialized(t *testing.T) {
	status := false
	payload, err := json.Marshal(&common.Function{
		Admin:         true,
		Cron:          map[string]string{"task": "*/5 * * * *"},
		Status:        &status,
		InstallStatus: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["admin"] != true {
		t.Fatalf("admin = %#v; want true", decoded["admin"])
	}
	if decoded["status"] != false || decoded["install_status"] != float64(2) {
		t.Fatalf("plugin status fields = %#v; want status=false and install_status=2", decoded)
	}
	cron, ok := decoded["cron"].(map[string]interface{})
	if !ok || cron["task"] != "*/5 * * * *" {
		t.Fatalf("cron = %#v; want task schedule", decoded["cron"])
	}
}

func TestGithubPublicIndexReadsMarketFlags(t *testing.T) {
	records, err := parseGithubPublicFileIndex([]byte(`{
  "plugins/timer.js": {
    "title": "Timer",
    "path": "plugins/timer.js",
    "type": "node",
    "admin": true,
    "module": true,
    "cron": "*/5 * * * *"
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !records[0].Admin || !records[0].Module || records[0].Cron != "*/5 * * * *" {
		t.Fatalf("records = %#v; want admin module timer metadata", records)
	}
}

func TestGithubPublicIndexSplitsPackageAndModuleDependencies(t *testing.T) {
	records, err := parseGithubPublicFileIndex([]byte(`{
  "plugins/consumer.js": {
    "title": "Consumer",
    "path": "plugins/consumer.js",
    "type": "node",
    "dependencies": ["axios", "./shared.js"],
    "module_dependencies": ["./logger.js"]
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(records[0].Dependencies) != 1 || records[0].Dependencies[0] != "axios" {
		t.Fatalf("package dependencies = %#v", records)
	}
	if strings.Join(records[0].ModuleDependencies, ",") != "./logger.js,./shared.js" {
		t.Fatalf("module dependencies = %#v", records[0].ModuleDependencies)
	}
}

func TestPrivatePluginTabKeepsGlobalMarketCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	active := true
	remote := &common.Function{
		UUID:     "remote-market-plugin",
		Title:    "Remote",
		Type:     NODE,
		Version:  "v1.0.0",
		CreateAt: time.Now().UTC().Format(time.RFC3339),
	}
	local := &common.Function{
		UUID:    "local-private-plugin",
		Title:   "Local",
		Type:    NODE,
		Version: "v1.0.0",
		Status:  &active,
	}
	previousMarket, previousFunctions := plugin_list, Functions
	plugin_list = []*common.Function{remote}
	Functions = []*common.Function{local}
	t.Cleanup(func() {
		plugin_list = previousMarket
		Functions = previousFunctions
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/plugin-market/plugins?page=1&page_size=10&status=private", nil)
	handlePluginMarketPlugins(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("market response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	response := struct {
		Data RequestPluginResult `json:"data"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	result := response.Data
	if result.All != 2 || result.Private != 1 || result.Tab1 != 1 || result.Tab2 != 1 || result.Latest != 1 {
		t.Fatalf("unexpected global counts on private tab: %#v", result)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Data[0].UUID != local.UUID {
		t.Fatalf("unexpected private tab data: %#v", result.Data)
	}
}

func TestDependencyModuleMarketTab(t *testing.T) {
	gin.SetMode(gin.TestMode)
	module := &common.Function{UUID: "shared-module", Title: "Shared", Type: NODE, Module: true, Version: "v1.0.0"}
	normal := &common.Function{UUID: "normal-plugin", Title: "Normal", Type: NODE, Version: "v1.0.0"}
	previousMarket, previousFunctions := plugin_list, Functions
	plugin_list = []*common.Function{module, normal}
	Functions = nil
	t.Cleanup(func() {
		plugin_list = previousMarket
		Functions = previousFunctions
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/plugin-market/plugins?page=1&page_size=10&status=module", nil)
	handlePluginMarketPlugins(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("market response HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	response := struct {
		Data RequestPluginResult `json:"data"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Modules != 1 || response.Data.Total != 1 || len(response.Data.Data) != 1 || !response.Data.Data[0].Module {
		t.Fatalf("unexpected module tab response: %#v", response.Data)
	}
}

func TestPluginMarketResponseDoesNotMutateCachedMarketItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	remote := &common.Function{UUID: "cached-remote", Title: "Remote", Type: NODE, Version: "v2.0.0", Status: &enabled}
	installed := &common.Function{UUID: remote.UUID, Title: "Remote", Type: NODE, Version: "v1.0.0", Status: &enabled}
	previousMarket, previousFunctions := plugin_list, Functions
	plugin_list = []*common.Function{remote}
	Functions = []*common.Function{installed}
	t.Cleanup(func() {
		plugin_list = previousMarket
		Functions = previousFunctions
	})

	request := func() RequestPluginResult {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/plugin-market/plugins?page=1&page_size=10", nil)
		handlePluginMarketPlugins(ctx)
		if recorder.Code != http.StatusOK {
			t.Fatalf("market response HTTP %d: %s", recorder.Code, recorder.Body.String())
		}
		response := struct {
			Data RequestPluginResult `json:"data"`
		}{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Data
	}

	first := request()
	if len(first.Data) != 1 || first.Data[0].InstallStatus != 1 {
		t.Fatalf("installed market response = %#v", first.Data)
	}
	if remote.InstallStatus != 0 || remote.CurrentVersion != "" || remote.LatestVersion != "" {
		t.Fatalf("cached market item was mutated: %#v", remote)
	}

	Functions = nil
	second := request()
	if len(second.Data) != 1 || second.Data[0].InstallStatus != 0 {
		t.Fatalf("uninstalled market response retained stale state: %#v", second.Data)
	}
}

func TestPluginModuleDependentsUseDeclaredDepeOnly(t *testing.T) {
	dir := t.TempDir()
	write := func(name, source string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	nodeModule := &common.Function{UUID: "node-module", Title: "shared-tools", Type: NODE, Module: true, Path: write("shared-tools.js", "module.exports = {}")}
	pythonModule := &common.Function{UUID: "python-module", Title: "shared_data", Type: PYTHON, Module: true, Path: write("shared_data.py", "VALUE = 1")}
	installed := []*common.Function{
		nodeModule,
		pythonModule,
		{UUID: "node-user", Title: "Node User", Type: NODE, Path: write("node-user.js", "// [depe: [\"./shared-tools.js\"]]\n")},
		{UUID: "python-user", Title: "Python User", Type: PYTHON, Path: write("python-user.py", "# [depe: [\"./shared_data.py\"]]\n")},
		{UUID: "require-only", Title: "Require Only", Type: NODE, Path: write("require-only.js", `const tools = require("./shared-tools");`)},
		{UUID: "import-only", Title: "Import Only", Type: PYTHON, Path: write("import-only.py", "from shared_data import VALUE\n")},
		{UUID: "unrelated", Title: "Unrelated", Type: NODE, Path: write("unrelated.js", `require("axios")`)},
	}

	nodeDependents, err := pluginModuleDependents(nodeModule, installed)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodeDependents) != 1 || nodeDependents[0].ID != "node-user" {
		t.Fatalf("node dependents = %#v", nodeDependents)
	}
	pythonDependents, err := pluginModuleDependents(pythonModule, installed)
	if err != nil {
		t.Fatal(err)
	}
	if len(pythonDependents) != 1 || pythonDependents[0].ID != "python-user" {
		t.Fatalf("python dependents = %#v", pythonDependents)
	}
	if err := ensurePluginModuleUnused(nodeModule, installed); err == nil || !strings.Contains(err.Error(), "Node User") {
		t.Fatalf("module uninstall guard error = %v", err)
	}
}

func TestInstallMarketPluginInstallsDeclaredModulesFirst(t *testing.T) {
	module := &common.Function{
		UUID: "shared", Title: "Shared", Type: NODE, Module: true,
		Path: filepath.Join("plugins", "shared.js"), Address: "module-address", Dependencies: []string{"lodash"},
	}
	plugin := &common.Function{
		UUID: "consumer", Title: "Consumer", Type: NODE,
		Path: filepath.Join("plugins", "consumer.js"), Address: "plugin-address",
		ModuleDependencies: []string{"./shared.js"}, Dependencies: []string{"axios"},
	}
	installed := []string{}
	err := installMarketPluginWithModuleDependencies(plugin, []*common.Function{plugin, module}, nil, func(address string) error {
		installed = append(installed, "file:"+address)
		return nil
	}, func(runtime, dependency string) error {
		installed = append(installed, "dependency:"+runtime+":"+dependency)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(installed, ",") != "dependency:node:lodash,file:module-address,dependency:node:axios,file:plugin-address" {
		t.Fatalf("install order = %#v", installed)
	}
}

func TestMissingPluginModuleDependenciesSkipsInstalledPublisherModule(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	consumer := &common.Function{
		UUID:               "consumer",
		Type:               NODE,
		Path:               filepath.Join(nodePluginsRoot(), "mrconli", "consumer.js"),
		ModuleDependencies: []string{"./mrconliAccountRuntime.js"},
	}
	module := &common.Function{
		UUID:   "account-runtime",
		Type:   NODE,
		Module: true,
		Path:   filepath.Join(nodePluginsRoot(), "mrconli", "mrconliAccountRuntime.js"),
	}
	if missing := missingPluginModuleDependencies(consumer, []*common.Function{consumer, module}); len(missing) != 0 {
		t.Fatalf("installed module was reported missing: %#v", missing)
	}

	module.Path = filepath.Join(nodePluginsRoot(), "another-author", "mrconliAccountRuntime.js")
	missing := missingPluginModuleDependencies(consumer, []*common.Function{consumer, module})
	if len(missing) != 1 || missing[0] != "./mrconliAccountRuntime.js" {
		t.Fatalf("cross-publisher module lookup = %#v", missing)
	}
}

func TestInstallMarketPluginDependenciesUsesDownloadedSourceAndSkipsInstalledFiles(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := &common.Function{
		UUID:         "consumer",
		Title:        "Consumer",
		Type:         NODE,
		Path:         filepath.Join(nodePluginsRoot(), "mrconli", "consumer.js"),
		Dependencies: []string{"source-package"},
		ModuleDependencies: []string{
			"./mrconliAccountRuntime.js",
		},
	}
	installedModule := &common.Function{
		UUID:   "account-runtime",
		Title:  "Account Runtime",
		Type:   NODE,
		Module: true,
		Path:   filepath.Join(nodePluginsRoot(), "mrconli", "mrconliAccountRuntime.js"),
	}
	marketRoot := &common.Function{
		UUID:         root.UUID,
		Title:        root.Title,
		Type:         NODE,
		Address:      "plugin-address",
		Dependencies: []string{"stale-market-package"},
	}
	fileInstalls := []string{}
	packageInstalls := []string{}
	dependencyRoot := downloadedPluginDependencyRoot(marketRoot, []*common.Function{root, installedModule})
	if strings.Join(dependencyRoot.Dependencies, ",") != "source-package" || dependencyRoot.Address != "plugin-address" {
		t.Fatalf("dependency root = %#v; want downloaded annotations plus market address", dependencyRoot)
	}
	err := installMarketPluginDependencies(
		dependencyRoot,
		[]*common.Function{marketRoot},
		[]*common.Function{root, installedModule},
		func(address string) error {
			fileInstalls = append(fileInstalls, address)
			return nil
		},
		func(runtime, dependency string) error {
			packageInstalls = append(packageInstalls, runtime+":"+dependency)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(fileInstalls) != 0 {
		t.Fatalf("installed root/module were downloaded again: %#v", fileInstalls)
	}
	if strings.Join(packageInstalls, ",") != "node:source-package" {
		t.Fatalf("runtime dependencies = %#v; want downloaded source only", packageInstalls)
	}
}

func TestInstallMarketPluginStopsBeforeFileWhenRuntimeDependencyFails(t *testing.T) {
	plugin := &common.Function{UUID: "consumer", Title: "Consumer", Type: NODE, Address: "plugin-address", Dependencies: []string{"broken-package"}}
	fileInstalled := false
	err := installMarketPluginWithModuleDependencies(plugin, []*common.Function{plugin}, nil, func(string) error {
		fileInstalled = true
		return nil
	}, func(string, string) error { return errors.New("registry failed") })
	if err == nil || !strings.Contains(err.Error(), "broken-package") {
		t.Fatalf("dependency failure = %v", err)
	}
	if fileInstalled {
		t.Fatal("plugin file must not be installed after runtime dependency failure")
	}
}

func TestInstallMarketPluginRejectsNonModuleAndCycles(t *testing.T) {
	nonModule := &common.Function{UUID: "shared", Title: "Shared", Type: NODE, Path: filepath.Join("plugins", "shared.js"), Address: "shared"}
	consumer := &common.Function{UUID: "consumer", Title: "Consumer", Type: NODE, Path: filepath.Join("plugins", "consumer.js"), Address: "consumer", ModuleDependencies: []string{"./shared.js"}}
	if err := installMarketPluginWithModuleDependencies(consumer, []*common.Function{consumer, nonModule}, nil, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "module:true") {
		t.Fatalf("non-module dependency error = %v", err)
	}

	a := &common.Function{UUID: "a", Title: "A", Type: NODE, Module: true, Path: filepath.Join("plugins", "a.js"), Address: "a", ModuleDependencies: []string{"./b.js"}}
	b := &common.Function{UUID: "b", Title: "B", Type: NODE, Module: true, Path: filepath.Join("plugins", "b.js"), Address: "b", ModuleDependencies: []string{"./a.js"}}
	if err := installMarketPluginWithModuleDependencies(a, []*common.Function{a, b}, nil, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "循环") {
		t.Fatalf("cycle error = %v", err)
	}
}
