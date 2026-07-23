package core

import (
	"os"
	"path/filepath"
	"testing"
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
	if len(deps) != 3 {
		t.Fatalf("unexpected dependencies: %#v", deps)
	}
	names := map[string]bool{}
	for _, dep := range deps {
		names[dep.Name] = true
	}
	for _, name := range []string{"ipp", "@grpc/grpc-js", "google-protobuf"} {
		if !names[name] {
			t.Fatalf("missing dependency %s in %#v", name, deps)
		}
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
