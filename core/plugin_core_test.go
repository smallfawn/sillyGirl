package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveNodePluginFilesRejectsPathOutsidePluginRoot(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	outside := filepath.Join(t.TempDir(), "main.js")
	if err := os.WriteFile(outside, []byte("console.log('outside')"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := removeNodePluginFiles(outside); err == nil {
		t.Fatal("removeNodePluginFiles outside root returned nil error")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file should remain: %v", err)
	}
}

func TestRemoveNodePluginFilesRemovesOnlySelectedPlugin(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	pluginDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(pluginDir, "main.js")
	if err := os.WriteFile(mainFile, []byte("console.log('demo')"), 0644); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(pluginDir, "sharedTools.js")
	if err := os.WriteFile(sibling, []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := removeNodePluginFiles(mainFile); err != nil {
		t.Fatalf("removeNodePluginFiles returned error: %v", err)
	}
	if _, err := os.Stat(mainFile); !os.IsNotExist(err) {
		t.Fatalf("selected plugin should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("sibling module should remain: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("plugin root should remain: %v", err)
	}
}
