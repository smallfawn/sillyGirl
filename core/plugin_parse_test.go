package core

import "testing"

func TestPluginParseDefaultIcon(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title demo
 * @rule ^demo$
 */`, "demo")
	if fn.Icon != defaultPluginIconURL {
		t.Fatalf("Icon = %q; want %q", fn.Icon, defaultPluginIconURL)
	}
}

func TestPluginParseKeepsDeclaredIcon(t *testing.T) {
	const icon = "https://example.com/custom.png"
	fn, _ := pluginParse(`/**
 * @title demo
 * @rule ^demo$
 * @icon https://example.com/custom.png
 */`, "demo")
	if fn.Icon != icon {
		t.Fatalf("Icon = %q; want %q", fn.Icon, icon)
	}
}

func TestPluginParseLegacyCommentMetadata(t *testing.T) {
	fn, _ := pluginParse(`//[title: 老式Node]
//[description: 老式说明]
// [rule: ^老式$]
// [version: v1.2.3]
// [author: tester]
// [class: 工具 任务]
// [public: true]
// [admin: true]
// [priority: 18]
// [param: {"key":"legacy.token","name":"Token"}]
`, "legacy-node")
	if fn.Title != "老式Node" || fn.Description != "老式说明" || fn.Version != "v1.2.3" || fn.Author != "tester" {
		t.Fatalf("legacy node metadata parse failed: %#v", fn)
	}
	if len(fn.Rules) != 1 || fn.Rules[0] != "^老式$" {
		t.Fatalf("legacy node rules = %#v", fn.Rules)
	}
	if !fn.Public || !fn.Admin || fn.Priority != 18 || fn.Class != "工具 任务" {
		t.Fatalf("legacy node flags parse failed: public=%v admin=%v priority=%d class=%q", fn.Public, fn.Admin, fn.Priority, fn.Class)
	}
	if fn.HasForm {
		t.Fatalf("legacy [param] must not register a config form")
	}

	py, _ := pluginParse(`##[title: 老式Python]
##[description: Python说明]
# [rule: ^py老式$]
# [cron: 12 8 * * *]
`, "legacy-python")
	if py.Title != "老式Python" || py.Description != "Python说明" {
		t.Fatalf("legacy python metadata parse failed: %#v", py)
	}
	if len(py.Rules) != 1 || py.Rules[0] != "^py老式$" {
		t.Fatalf("legacy python rules = %#v", py.Rules)
	}
	if py.Cron["task"] != "12 8 * * *" {
		t.Fatalf("legacy python cron = %#v", py.Cron)
	}
}

func TestPluginParseLegacyCommentMetadataWithTrailingHelp(t *testing.T) {
	fn, _ := pluginParse(`//[title: 尾注Node] 这里是给旧市场看的字段说明
//[description: 尾注说明] 使用方法尽量写具体
//[version: v1.0.0] 版本格式说明
//[rule: ^尾注$] 指令说明
//[public: YES] 是否公开
`, "legacy-trailing-node")
	if fn.Title != "尾注Node" || fn.Description != "尾注说明" || fn.Version != "v1.0.0" {
		t.Fatalf("legacy trailing metadata parse failed: %#v", fn)
	}
	if len(fn.Rules) != 1 || fn.Rules[0] != "^尾注$" {
		t.Fatalf("legacy trailing rule = %#v", fn.Rules)
	}
	if !fn.Public {
		t.Fatalf("legacy trailing public flag parse failed: %#v", fn)
	}
}

func TestPluginParsePythonDocstringAtMetadata(t *testing.T) {
	fn, _ := pluginParse(`"""
@title Python旧式
@description 三引号说明
@rule ^pynew$
@version v1.2.4
@public true
@depe ["requests"]
"""
`, "python-docstring-meta")
	if fn.Title != "Python旧式" || fn.Description != "三引号说明" || fn.Version != "v1.2.4" {
		t.Fatalf("python docstring metadata parse failed: %#v", fn)
	}
	if len(fn.Rules) != 1 || fn.Rules[0] != "^pynew$" || !fn.Public {
		t.Fatalf("python docstring rule/flag parse failed: %#v", fn)
	}
}

func TestPluginParseAtMetadataEOFAndBoolVariants(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title EOFMeta
 * @rule ^eof$
 * @public YES
 * @admin 1
 * @module on
 * @carry
 * @smallcat TRUE
 */`, "eof-meta")
	if fn.Title != "EOFMeta" || len(fn.Rules) != 1 || fn.Rules[0] != "^eof$" {
		t.Fatalf("metadata at EOF parse failed: %#v", fn)
	}
	if !fn.Public || !fn.Admin || !fn.Module || !fn.Carry || !fn.UsesSmallCat {
		t.Fatalf("bool variants parse failed: public=%v admin=%v module=%v carry=%v smallcat=%v", fn.Public, fn.Admin, fn.Module, fn.Carry, fn.UsesSmallCat)
	}
}

func TestPluginParseDetectsSmallCatUsage(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "javascript constructor", script: `const client = new ct.SmallCat({id: 1})`, want: true},
		{name: "python constructor", script: `client = SmallCat({"id": 1})`, want: true},
		{name: "static call", script: `SmallCat.userList()`, want: true},
		{name: "description only", script: `/** SmallCat account helper */`, want: false},
		{name: "metadata override", script: "/**\n * @smallcat true\n */", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fn, _ := pluginParse(test.script, "demo")
			if fn.UsesSmallCat != test.want {
				t.Fatalf("UsesSmallCat = %v; want %v", fn.UsesSmallCat, test.want)
			}
		})
	}
}

func TestPluginParseDetectsV2FormUsage(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "new form", script: `const config = new form({ token: form.string() })`, want: true},
		{name: "spaced new form", script: `const config = new form ({ token: form.string() })`, want: true},
		{name: "python form", script: `config = form({"token": form.string()})`, want: true},
		{name: "word only", script: `// form config`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fn, _ := pluginParse(test.script, "demo")
			if fn.HasForm != test.want {
				t.Fatalf("HasForm = %v; want %v", fn.HasForm, test.want)
			}
		})
	}
}
