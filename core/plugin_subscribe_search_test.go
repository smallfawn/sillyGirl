package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestPluginMatchesKeyword(t *testing.T) {
	plugin := &common.Function{
		UUID:            "jdCodeLogin",
		Title:           "京东CODE登录",
		Description:     "通过 SmallCat OAuth 获取 Cookie",
		Author:          "SmallFawn",
		Class:           "工具",
		PluginPublisher: common.PluginPublisher{Organization: "Example/Plugin-Market"},
		Rule:            "^京东登录$",
		Dependencies:    []string{"Axios-Extra"},
	}
	tests := []struct {
		name    string
		keyword string
		want    bool
	}{
		{name: "empty", keyword: "", want: true},
		{name: "case insensitive title", keyword: "code", want: true},
		{name: "case insensitive author", keyword: "smallfawn", want: true},
		{name: "separator insensitive", keyword: "jdcodelogin", want: true},
		{name: "multiple fuzzy tokens", keyword: "smallcat cookie", want: true},
		{name: "source", keyword: "plugin market", want: true},
		{name: "dependency", keyword: "axios extra", want: true},
		{name: "missing", keyword: "telegram weather", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := pluginMatchesKeyword(plugin, test.keyword); got != test.want {
				t.Fatalf("pluginMatchesKeyword(%q) = %v, want %v", test.keyword, got, test.want)
			}
		})
	}
}

func TestPluginMatchesKeywordNil(t *testing.T) {
	if pluginMatchesKeyword(nil, "code") {
		t.Fatal("nil plugin unexpectedly matched")
	}
}
