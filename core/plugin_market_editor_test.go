package core

import (
	"strings"
	"testing"
)

func TestValidateAndNormalizeLocalPluginContentAtStyle(t *testing.T) {
	content := `/**
 * @title Demo
 * @name demo
 * @description demo desc
 * @version v1.0.0
 * @rule ^demo$
 * @public true
 */
console.log("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "* @public false") {
		t.Fatalf("public flag was not forced to false for @ metadata:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentPythonAtStyle(t *testing.T) {
	content := `"""
@title PyDemo
@name pyDemo
@description py desc
@version v1.0.0
@cron 12 8 * * *
"""
print("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "@public false") {
		t.Fatalf("public flag was not inserted for Python @ metadata:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentNewStyleNoSpace(t *testing.T) {
	content := `//[title: Demo]
//[name: demo]
//[description: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
//[public: true]

console.log("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false]") {
		t.Fatalf("public flag was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentHashNewStyle(t *testing.T) {
	content := `##[title: PyDemo]
##[name: pyDemo]
##[description: py desc]
##[version: v1.0.0]
##[cron: 12 8 * * *]

print("ok")
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "#[public: false]") {
		t.Fatalf("public flag was not inserted for hash metadata:\n%s", got)
	}
}

func TestForceLocalPluginPrivateBoolVariants(t *testing.T) {
	content := `//[title: Demo]
//[name: demo]
//[description: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
//[public: YES]
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false]") {
		t.Fatalf("public YES was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentLegacyTrailingHelp(t *testing.T) {
	content := `//[title: Demo] 插件标题
//[name: demo] 文件名
//[description: demo desc] 说明文字
//[version: v1.0.0] 版本说明
//[rule: ^demo$] 指令说明
//[public: YES] 是否公开
`
	got, err := validateAndNormalizeLocalPluginContent(content)
	if err != nil {
		t.Fatalf("validateAndNormalizeLocalPluginContent error: %v", err)
	}
	if !strings.Contains(got, "//[public: false] 是否公开") {
		t.Fatalf("public YES with trailing help was not forced to false:\n%s", got)
	}
}

func TestValidateAndNormalizeLocalPluginContentRequiresName(t *testing.T) {
	content := `//[title: Demo]
//[description: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
`
	if _, err := validateAndNormalizeLocalPluginContent(content); err == nil || !strings.Contains(err.Error(), "[name: 文件名]") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestValidateLocalPluginRequestNameMatchesMetaName(t *testing.T) {
	content := `//[title: Demo]
//[name: demoPlugin]
//[description: demo desc]
//[version: v1.0.0]
//[rule: ^demo$]
`
	if err := validateLocalPluginRequestName("demoPlugin.js", content, NODE); err != nil {
		t.Fatalf("expected matching plugin name, got %v", err)
	}
	if err := validateLocalPluginRequestName("otherPlugin", content, NODE); err == nil || !strings.Contains(err.Error(), "必须和 [name: demoplugin] 一致") {
		t.Fatalf("expected name mismatch error, got %v", err)
	}
}
