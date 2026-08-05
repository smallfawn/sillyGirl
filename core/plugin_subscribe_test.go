package core

import (
	"path"
	"reflect"
	"testing"
)

func TestParseGithubPublicFileIndexReadsDependenciesField(t *testing.T) {
	data := []byte(`{
  "fbd8dead-f6ca-56e8-8293-d7980d1bbf91": {
    "id": "fbd8dead-f6ca-56e8-8293-d7980d1bbf91",
    "title": "getPrinterStatus",
    "author": "smallfawn",
    "version": "v1.0.0",
    "desc": "定时获取打印机状态",
    "icon": "https://example.com/printer.png",
    "class": "工具",
    "path": "plugins/getPrinterStatus.js",
    "raw": "https://raw.githubusercontent.com/smallfawn/sillyGirl_Plugins/main/plugins/getPrinterStatus.js",
    "dependencies": ["ipp", "axios", "node:fs", "sillygirl", "ipp"],
    "type": "node",
    "origin": "https://github.com/smallfawn/sillyGirl_Plugins"
  }
}`)

	items, err := parseGithubPublicFileIndex(data)
	if err != nil {
		t.Fatalf("parseGithubPublicFileIndex() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Desc != "定时获取打印机状态" {
		t.Fatalf("Desc = %q", item.Desc)
	}
	if item.Class != "工具" {
		t.Fatalf("Class = %q", item.Class)
	}
	if item.Icon != "https://example.com/printer.png" {
		t.Fatalf("Icon = %q", item.Icon)
	}
	wantDeps := []string{"axios", "ipp"}
	if !reflect.DeepEqual(item.Dependencies, wantDeps) {
		t.Fatalf("Dependencies = %#v; want %#v", item.Dependencies, wantDeps)
	}
}

func TestGithubPublicFileIndexPythonPlugin(t *testing.T) {
	source := &githubPluginSource{
		Owner:  "smallfawn",
		Repo:   "sillyGirl_Plugins",
		Branch: "main",
	}
	data := []byte(`{
  "plugins/pythonDemo.py": {
    "title": "Python Demo",
    "version": "v1.0.0",
    "desc": "Python 示例",
    "class": "工具",
    "type": "python",
    "path": "plugins/pythonDemo.py",
    "raw": "https://raw.githubusercontent.com/smallfawn/sillyGirl_Plugins/main/plugins/pythonDemo.py",
    "dependencies": ["requests==2.32.0", "beautiful_soup4", "os"]
  }
}`)

	records, err := parseGithubPublicFileIndex(data)
	if err != nil {
		t.Fatalf("parseGithubPublicFileIndex() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	wantDeps := []string{"beautiful-soup4", "requests"}
	if !reflect.DeepEqual(record.Dependencies, wantDeps) {
		t.Fatalf("Dependencies = %#v; want %#v", record.Dependencies, wantDeps)
	}
	if !isGithubFlatNodePlugin(record.Path) {
		t.Fatalf("isGithubFlatNodePlugin(%q) = false, want true", record.Path)
	}
	class := pluginClassFromExt(path.Ext(record.Path))
	if class != PYTHON {
		t.Fatalf("pluginClassFromExt(%q) = %q; want %q", record.Path, class, PYTHON)
	}
	address := makeGithubNodePluginAddress(source, record.Path, record.Raw, PYTHON)
	parsed, pluginPath, rawURL, class, err := parseGithubNodePluginAddress(address)
	if err != nil {
		t.Fatalf("parseGithubNodePluginAddress() error = %v", err)
	}
	if parsed.Owner != source.Owner || parsed.Repo != source.Repo || parsed.Branch != source.Branch {
		t.Fatalf("parsed source = %#v; want %#v", parsed, source)
	}
	if pluginPath != record.Path || rawURL != record.Raw {
		t.Fatalf("parsed address = (%q, %q); want (%q, %q)", pluginPath, rawURL, record.Path, record.Raw)
	}
	if class != PYTHON {
		t.Fatalf("parsed class = %q; want %q", class, PYTHON)
	}
}

func TestParseDeclaredDependenciesFromDepeComment(t *testing.T) {
	nodeDeps := parseDeclaredDependencies(`/**
 * @title printer
 * @depe ["ipp", "axios"]
 */
const ipp = require("ipp");
`, NODE)
	if len(nodeDeps) != 2 || nodeDeps[0] != "axios" || nodeDeps[1] != "ipp" {
		t.Fatalf("parseDeclaredDependencies node = %#v; want [axios ipp]", nodeDeps)
	}
	pythonDeps := parseDeclaredDependencies(`"""
 * @title py
 * @depe ["requests==2.32.0", "beautiful_soup4"]
"""
import requests
`, PYTHON)
	if len(pythonDeps) != 2 || pythonDeps[0] != "beautiful-soup4" || pythonDeps[1] != "requests" {
		t.Fatalf("parseDeclaredDependencies python = %#v; want [beautiful-soup4 requests]", pythonDeps)
	}
}

func TestParseDeclaredDependenciesFromLegacyComment(t *testing.T) {
	nodeDeps := parseDeclaredDependencies(`//[title: old node]
//[depe: ["ipp", "axios"]]
const ipp = require("ipp");
`, NODE)
	if len(nodeDeps) != 2 || nodeDeps[0] != "axios" || nodeDeps[1] != "ipp" {
		t.Fatalf("parseDeclaredDependencies legacy node = %#v; want [axios ipp]", nodeDeps)
	}
	pythonDeps := parseDeclaredDependencies(`##[title: old python]
##[depe: {"requests==2.32.0":"", "beautiful_soup4":""}]
import requests
`, PYTHON)
	if len(pythonDeps) != 2 || pythonDeps[0] != "beautiful-soup4" || pythonDeps[1] != "requests" {
		t.Fatalf("parseDeclaredDependencies legacy python = %#v; want [beautiful-soup4 requests]", pythonDeps)
	}
}

func TestPluginClassFromIndexTypeHasPriority(t *testing.T) {
	if got := pluginClassFromIndexType("python", "plugins/demo.js"); got != PYTHON {
		t.Fatalf("pluginClassFromIndexType python = %q; want %q", got, PYTHON)
	}
	if got := pluginClassFromIndexType("node", "plugins/demo.py"); got != NODE {
		t.Fatalf("pluginClassFromIndexType node = %q; want %q", got, NODE)
	}
	if got := pluginClassFromIndexType("", "plugins/demo.py"); got != PYTHON {
		t.Fatalf("pluginClassFromIndexType fallback = %q; want %q", got, PYTHON)
	}
}
