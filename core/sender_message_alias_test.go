package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	proto3assets "github.com/smallfawn/sillyGirl/proto3"
)

func TestEmbeddedSenderAPI(t *testing.T) {
	nodeRuntime, err := proto3assets.ReadRuntimeFile("sillygirl.js")
	if err != nil {
		t.Fatal(err)
	}
	nodeTypes, err := proto3assets.ReadRuntimeFile("sillygirl.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	pythonRuntime, err := proto3assets.ReadRuntimeFile("sillygirl.py")
	if err != nil {
		t.Fatal(err)
	}

	for label, source := range map[string]string{
		"node runtime":    string(nodeRuntime),
		"node types":      string(nodeTypes),
		"generated types": typeat,
		"python runtime":  string(pythonRuntime),
	} {
		for _, method := range []string{"getMsg", "setMsg", "getMsgId"} {
			if !strings.Contains(source, method) {
				t.Errorf("%s is missing %s", label, method)
			}
		}
	}

	nodeMessageMethod := string(nodeRuntime)
	start := strings.Index(nodeMessageMethod, "async getMsg()")
	end := strings.Index(nodeMessageMethod, "async isAdmin()")
	if start < 0 || end <= start || !strings.Contains(nodeMessageMethod[start:end], "client.SenderGetContent") {
		t.Fatal("node getMsg does not call SenderGetContent")
	}
	if !strings.Contains(nodeMessageMethod, "async getMsgId()") || !strings.Contains(nodeMessageMethod, "client.SenderGetMessageId") {
		t.Fatal("node getMsgId does not call SenderGetMessageId")
	}
	pythonMessageMethod := string(pythonRuntime)
	start = strings.Index(pythonMessageMethod, "async def getMsg(self):")
	end = strings.Index(pythonMessageMethod, "async def isAdmin(self):")
	if start < 0 || end <= start || !strings.Contains(pythonMessageMethod[start:end], "SenderGetContent") {
		t.Fatal("python getMsg does not call SenderGetContent")
	}
	if !strings.Contains(pythonMessageMethod, "async def getMsgId(self):") || !strings.Contains(pythonMessageMethod, "SenderGetMessageId") {
		t.Fatal("python getMsgId does not call SenderGetMessageId")
	}
	if !strings.Contains(nodeMessageMethod, "async setMsg(content)") || !strings.Contains(nodeMessageMethod, "client.SenderSetContent") {
		t.Fatal("node setMsg does not call SenderSetContent")
	}
	if !strings.Contains(pythonMessageMethod, "async def setMsg(self, content):") || !strings.Contains(pythonMessageMethod, "SenderSetContent") {
		t.Fatal("python setMsg does not call SenderSetContent")
	}

	pluginHome := t.TempDir()
	if err := ensureNodeSillygirlModule(pluginHome); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		filepath.Join("node_modules", "sillygirl", "index.js"),
		filepath.Join("node_modules", "sillygirl", "sillygirl.d.ts"),
		filepath.Join("node_modules", "sillygirl.d.ts"),
	} {
		installed, err := os.ReadFile(filepath.Join(pluginHome, relative))
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"getMsg", "setMsg", "getMsgId"} {
			if !strings.Contains(string(installed), method) {
				t.Errorf("installed runtime %s is missing %s", relative, method)
			}
		}
	}
}
