package core

import "testing"

func TestParseGithubPublicFileIndexObjectDependencies(t *testing.T) {
	data := []byte(`{
  "fbd8dead-f6ca-56e8-8293-d7980d1bbf91": {
    "id": "fbd8dead-f6ca-56e8-8293-d7980d1bbf91",
    "title": "getPrinterStatus",
    "author": "smallfawn",
    "version": "v1.0.0",
    "desc": "定时获取打印机状态",
    "class": "工具",
    "path": "plugins/getPrinterStatus.js",
    "raw": "https://raw.githubusercontent.com/smallfawn/sillyGirl_Plugins/main/plugins/getPrinterStatus.js",
    "dependencies": {
      "ipp": "latest"
    },
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
	if got := []string(item.Dependencies); len(got) != 1 || got[0] != "ipp" {
		t.Fatalf("Dependencies = %#v, want [ipp]", got)
	}
}
