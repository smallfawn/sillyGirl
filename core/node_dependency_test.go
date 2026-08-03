package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	proto3assets "github.com/smallfawn/sillyGirl/proto3"
)

func TestEnsureNodePackageJSONRepairsInvalidDependencyFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(`{"name":"bad","version":"1.0.0","dependencies":{"ipp":"^2.0.1"},"devDependencies":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ensureNodePackageJSON(dir, "bad"); err != nil {
		t.Fatalf("ensureNodePackageJSON returned error: %v", err)
	}

	deps, err := readNodeDependencies(nodeDependencyPlugin{Name: "bad", Title: "bad", File: "main.js", Path: dir})
	if err != nil {
		t.Fatalf("readNodeDependencies returned error: %v", err)
	}
	if len(deps) != len(nodeSillygirlRuntimeDependencies)+1 {
		t.Fatalf("unexpected dependencies: %#v", deps)
	}
	names := map[string]bool{}
	for _, dep := range deps {
		names[dep.Name] = true
	}
	for name := range nodeSillygirlRuntimeDependencies {
		if !names[name] {
			t.Fatalf("missing dependency %s in %#v", name, deps)
		}
	}
	if !names["ipp"] {
		t.Fatalf("missing dependency ipp in %#v", deps)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]interface{}{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest["pnpm"]; ok {
		t.Fatalf("unexpected deprecated pnpm settings in %s", string(data))
	}
	workspace, err := os.ReadFile(filepath.Join(dir, "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workspace), "allowBuilds:\n  protobufjs: true\n") {
		t.Fatalf("missing protobufjs allowBuilds in %s", string(workspace))
	}
}

func TestEnsureNodePackageJSONCreatesPnpmBuildAllowlist(t *testing.T) {
	dir := t.TempDir()
	if err := ensureNodePackageJSON(dir, "new-plugin"); err != nil {
		t.Fatalf("ensureNodePackageJSON returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := map[string]interface{}{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest["pnpm"]; ok {
		t.Fatalf("unexpected deprecated pnpm settings in %s", string(data))
	}
	workspace, err := os.ReadFile(filepath.Join(dir, "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workspace), "allowBuilds:\n  protobufjs: true\n") {
		t.Fatalf("missing protobufjs allowBuilds in %s", string(workspace))
	}
}

func TestNodeRuntimeDependenciesIncludeGrpcPackages(t *testing.T) {
	for _, name := range []string{"@grpc/grpc-js", "google-protobuf"} {
		if _, ok := nodeSillygirlRuntimeDependencies[name]; !ok {
			t.Fatalf("missing runtime dependency %s", name)
		}
	}
	webFramework := "ex" + "press"
	if _, ok := nodeSillygirlRuntimeDependencies[webFramework]; ok {
		t.Fatal("web framework should not be installed as a built-in runtime dependency")
	}
}

func TestPnpmCommandArgsSkipsRegistryForRemove(t *testing.T) {
	registry := "https://registry.example.test"
	removeArgs := pnpmCommandArgs([]string{"pnpm.cjs"}, []string{"remove", "ipp"}, registry)
	if strings.Contains(strings.Join(removeArgs, " "), "--registry") {
		t.Fatalf("pnpm remove must not receive --registry: %#v", removeArgs)
	}
	addArgs := pnpmCommandArgs([]string{"pnpm.cjs"}, []string{"add", "ipp"}, registry)
	if got := strings.Join(addArgs, " "); !strings.Contains(got, "--registry "+registry) {
		t.Fatalf("pnpm add should receive registry override: %#v", addArgs)
	}
}

func TestEnsureNodeSillygirlModuleWritesRuntimeFiles(t *testing.T) {
	dir := t.TempDir()
	if err := ensureNodeSillygirlModule(dir); err != nil {
		t.Fatalf("ensureNodeSillygirlModule returned error: %v", err)
	}
	for _, name := range []string{
		filepath.Join("node_modules", "sillygirl", "index.js"),
		filepath.Join("node_modules", "sillygirl", "srpc.js"),
		filepath.Join("node_modules", "sillygirl", "sillygirl.d.ts"),
		filepath.Join("node_modules", "sillygirl", "package.json"),
		filepath.Join("node_modules", "sillygirl.d.ts"),
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected runtime file %s: %v", name, err)
		}
	}
}

func TestEnsureNodeSillygirlModuleWorksWithoutProto3Directory(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})

	dir := t.TempDir()
	if err := ensureNodeSillygirlModule(dir); err != nil {
		t.Fatalf("ensureNodeSillygirlModule returned error without proto3 directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "sillygirl", "index.js")); err != nil {
		t.Fatalf("missing embedded node runtime index.js: %v", err)
	}
}

func TestEmbeddedRuntimeFilesAvailable(t *testing.T) {
	for _, name := range []string{
		"sillygirl.js",
		"srpc.js",
		"sillygirl.d.ts",
		"sillygirl.py",
		"srpc_pb2.py",
		"srpc_pb2_grpc.py",
	} {
		data, err := proto3assets.ReadRuntimeFile(name)
		if err != nil {
			t.Fatalf("missing embedded runtime file %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("embedded runtime file %s is empty", name)
		}
	}
}

func TestNormalizeNodeScriptFileName(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{name: "daily-sign", want: "daily-sign.js", ok: true},
		{name: "daily-sign.js", want: "daily-sign.js", ok: true},
		{name: "bad.ts", ok: false},
		{name: "../bad.js", ok: false},
		{name: "bad/name.js", ok: false},
	}
	for _, tt := range tests {
		got, err := normalizeNodeScriptFileName(tt.name)
		if tt.ok && err != nil {
			t.Fatalf("normalizeNodeScriptFileName(%q) returned error: %v", tt.name, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("normalizeNodeScriptFileName(%q) expected error, got %q", tt.name, got)
		}
		if tt.ok && got != tt.want {
			t.Fatalf("normalizeNodeScriptFileName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFinishNodeDependencyInstallRetriesPluginConfigSchema(t *testing.T) {
	if _, err := resolveNodeCommand(); err != nil {
		t.Skipf("node not available: %v", err)
	}

	dataHome := t.TempDir()
	t.Setenv("SILLYGIRL_DATA_PATH", dataHome)
	root := filepath.Join(dataHome, "plugins")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	pluginName := "dependency-form-retry"
	uuid := nameUuid(pluginName)
	pluginConfigSchemas.Set(uuid, "")
	t.Cleanup(func() {
		pluginConfigSchemas.Set(uuid, "")
	})
	pluginPath := filepath.Join(root, pluginName+".js")
	plugin := `
/**
 * @title Dependency form retry
 * @form true
 */
const helper = require("schema-helper-retry");
if (typeof helper.schemaTitle !== "string") {
  throw new Error("schema helper dependency is unavailable");
}
const { sillyGirlCreateSchema, SillyGirlPluginConfig } = require("sillygirl");
new SillyGirlPluginConfig(sillyGirlCreateSchema.object({
  token: sillyGirlCreateSchema.string().setTitle(helper.schemaTitle)
}));
`
	if err := os.WriteFile(pluginPath, []byte(plugin), 0644); err != nil {
		t.Fatal(err)
	}

	registrationErr := registerNodePluginConfigSchema(pluginPath, uuid)
	if registrationErr == nil {
		t.Fatal("config registration should fail before the dependency is installed")
	}
	t.Logf("baseline without dependency: %v", registrationErr)
	if got := pluginConfigSchemas.GetString(uuid); got != "" {
		t.Fatalf("schema should still be empty before dependency install: %s", got)
	}

	moduleDir := filepath.Join(root, "node_modules", "schema-helper-retry")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte(`module.exports = { schemaTitle: "Dependency Token" };`), 0644); err != nil {
		t.Fatal(err)
	}

	output := finishNodeDependencyInstall(root, pluginName, "dependency installed")
	if output != "dependency installed" {
		t.Fatalf("unexpected post-install warning: %s", output)
	}
	got := pluginConfigSchemas.GetString(uuid)
	if !strings.Contains(got, `"token"`) || !strings.Contains(got, `"Dependency Token"`) {
		t.Fatalf("schema was not retried after dependency install: %s", got)
	}
	t.Logf("modified after dependency install: %s", got)
}

func TestFinishPythonDependencyInstallRetriesPluginConfigSchema(t *testing.T) {
	if _, _, err := resolvePythonCommand(); err != nil {
		t.Skipf("python 3.12 not available: %v", err)
	}
	if _, err := resolvePipxCommand(); err != nil {
		t.Skipf("pipx not available: %v", err)
	}

	dataHome := t.TempDir()
	t.Setenv("SILLYGIRL_DATA_PATH", dataHome)
	root := filepath.Join(dataHome, "plugins")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	pluginName := "python-dependency-form-retry"
	uuid := nameUuid(pluginName)
	pluginConfigSchemas.Set(uuid, "")
	t.Cleanup(func() {
		pluginConfigSchemas.Set(uuid, "")
	})
	pluginPath := filepath.Join(root, pluginName+".py")
	plugin := `
"""
@title Python dependency form retry
@form true
"""
import schema_helper_retry

if schema_helper_retry.SCHEMA_TITLE != "Dependency Token":
    raise RuntimeError("schema helper dependency is unavailable")

from sillygirl import sillyGirlCreateSchema, SillyGirlPluginConfig

SillyGirlPluginConfig(sillyGirlCreateSchema.object({
    "token": sillyGirlCreateSchema.string().setTitle(schema_helper_retry.SCHEMA_TITLE)
}))
`
	if err := os.WriteFile(pluginPath, []byte(plugin), 0644); err != nil {
		t.Fatal(err)
	}

	registrationErr := registerPythonPluginConfigSchema(pluginPath, uuid)
	if registrationErr == nil {
		t.Fatal("config registration should fail before the Python dependency is installed")
	}
	t.Logf("baseline without Python dependency: %v", registrationErr)
	if got := pluginConfigSchemas.GetString(uuid); got != "" {
		t.Fatalf("schema should still be empty before Python dependency install: %s", got)
	}

	moduleDir := pythonPackagesDir()
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatal(err)
	}
	modulePath := filepath.Join(moduleDir, "schema_helper_retry.py")
	if err := os.WriteFile(modulePath, []byte(`SCHEMA_TITLE = "Dependency Token"`), 0644); err != nil {
		t.Fatal(err)
	}

	output := finishPythonDependencyInstall(pluginName, "dependency installed")
	if output != "dependency installed" {
		t.Fatalf("unexpected Python post-install warning: %s", output)
	}
	got := pluginConfigSchemas.GetString(uuid)
	if !strings.Contains(got, `"token"`) || !strings.Contains(got, `"Dependency Token"`) {
		t.Fatalf("Python schema was not retried after dependency install: %s", got)
	}
	t.Logf("modified after Python dependency install: %s", got)
}

func TestNormalizePythonDependencyName(t *testing.T) {
	tests := map[string]string{
		"requests==2.32.0":   "requests",
		"pydantic[email]":    "pydantic",
		"beautiful_soup4":    "beautiful-soup4",
		"urllib.parse":       "",
		"../bad":             "",
		"https://bad/pkg.py": "",
	}
	for input, want := range tests {
		if got := normalizePythonDependencyName(input); got != want {
			t.Fatalf("normalizePythonDependencyName(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestNormalizePipxRegistryDefault(t *testing.T) {
	got, err := normalizePipxRegistry("")
	if err != nil {
		t.Fatalf("normalizePipxRegistry returned error: %v", err)
	}
	if got != defaultPipxRegistry {
		t.Fatalf("normalizePipxRegistry(\"\") = %q; want %q", got, defaultPipxRegistry)
	}
}

func TestPythonRuntimeDependencyRequiresGeneratedVersions(t *testing.T) {
	if pythonGrpcRuntimeDependency != "grpcio==1.83.0" {
		t.Fatalf("pythonGrpcRuntimeDependency = %q", pythonGrpcRuntimeDependency)
	}
	if pythonProtobufRuntimeDependency != "protobuf==7.35.1" {
		t.Fatalf("pythonProtobufRuntimeDependency = %q", pythonProtobufRuntimeDependency)
	}
	if pythonRuntimeDependencyInstalled(pythonProtobufRuntimeDependency, map[string]string{"protobuf": "7.35.0"}) {
		t.Fatal("protobuf 7.35.0 should not satisfy the Python runtime protobuf constraint")
	}
	if !pythonRuntimeDependencyInstalled(pythonProtobufRuntimeDependency, map[string]string{"protobuf": "7.35.1"}) {
		t.Fatal("protobuf 7.35.1 should satisfy the Python runtime protobuf constraint")
	}
	if pythonRuntimeDependencyInstalled(pythonProtobufRuntimeDependency, map[string]string{"protobuf": "8.0.0"}) {
		t.Fatal("protobuf 8.0.0 should not satisfy the pinned Python runtime protobuf constraint")
	}
	if pythonRuntimeDependencyInstalled(pythonGrpcRuntimeDependency, map[string]string{"grpcio": "1.82.0"}) {
		t.Fatal("grpcio 1.82.0 should not satisfy the Python runtime grpcio constraint")
	}
	if !pythonRuntimeDependencyInstalled(pythonGrpcRuntimeDependency, map[string]string{"grpcio": "1.83.0"}) {
		t.Fatal("grpcio 1.83.0 should satisfy the Python runtime grpcio constraint")
	}
}
