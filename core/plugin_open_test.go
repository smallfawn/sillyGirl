package core

import (
	"reflect"
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestLocalPrivatePluginsExcludesRemoteAndNonScriptFunctions(t *testing.T) {
	remote := []*common.Function{{UUID: "remote", Title: "远程插件"}}
	installed := []*common.Function{
		{UUID: "remote", Title: "远程插件", Type: NODE},
		{UUID: "local-node", Title: "本地 Node", Type: NODE, Version: "v1.2.3", Dependencies: []string{"axios"}},
		{UUID: "local-python", Title: "本地 Python", Type: PYTHON},
		{UUID: "builtin", Title: "内置函数", Type: "go"},
		{Title: "无 UUID", Type: NODE},
	}

	rows := localPrivatePlugins(remote, installed)
	if len(rows) != 2 {
		t.Fatalf("localPrivatePlugins returned %d rows; want 2: %#v", len(rows), rows)
	}
	if rows[0].UUID != "local-node" || rows[1].UUID != "local-python" {
		t.Fatalf("unexpected private plugins: %#v", rows)
	}
	if rows[0].Status != 2 || rows[0].Organization != "本地插件" || rows[0].CurrentVersion != "v1.2.3" {
		t.Fatalf("local plugin market fields not initialized: %#v", rows[0])
	}
	rows[0].Dependencies[0] = "changed"
	if installed[1].Dependencies[0] != "axios" {
		t.Fatal("localPrivatePlugins must clone dependency slices")
	}
}

func TestOpenPluginRecordsOnlyReturnsEnabledOpenScripts(t *testing.T) {
	plugins := []*common.Function{
		{UUID: "open", Title: "开放插件", Type: NODE, Open: true, UsesSmallCat: true, Version: "v1.0.0", Dependencies: []string{"ipp"}},
		{UUID: "no-smallcat", Title: "普通插件", Type: NODE, Open: true},
		{UUID: "closed", Title: "未开放", Type: NODE},
		{UUID: "disabled", Title: "已停用", Type: PYTHON, Open: true, Disable: true},
		{UUID: "builtin", Title: "内置函数", Type: "go", Open: true},
	}

	rows := openPluginRecords(plugins)
	if len(rows) != 1 || rows[0].ID != "open" {
		t.Fatalf("unexpected open plugin records: %#v", rows)
	}
	if !reflect.DeepEqual(rows[0].Dependencies, []string{"ipp"}) {
		t.Fatalf("dependencies = %#v", rows[0].Dependencies)
	}
	rows[0].Dependencies[0] = "changed"
	if plugins[0].Dependencies[0] != "ipp" {
		t.Fatal("openPluginRecords must clone dependency slices")
	}
}

func TestPluginMarketCounts(t *testing.T) {
	market := []*common.Function{
		{UUID: "same", Version: "v1"},
		{UUID: "update", Version: "v2"},
		{UUID: "missing", Version: "v1"},
	}
	installed := []*common.Function{
		{UUID: "same", Version: "v1"},
		{UUID: "update", Version: "v1"},
		{UUID: "local", Version: "v1"},
	}
	gotInstalled, gotMissing, gotUpdates := pluginMarketCounts(market, installed)
	if gotInstalled != 2 || gotMissing != 1 || gotUpdates != 1 {
		t.Fatalf("pluginMarketCounts = (%d, %d, %d); want (2, 1, 1)", gotInstalled, gotMissing, gotUpdates)
	}
}
