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
	if len(deps) != 1 || deps[0].Name != "ipp" {
		t.Fatalf("unexpected dependencies: %#v", deps)
	}
}
