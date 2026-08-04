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

func TestPluginParseDetectsSmallCatUsage(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "javascript constructor", script: `const client = new SmallCat({id: 1})`, want: true},
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
