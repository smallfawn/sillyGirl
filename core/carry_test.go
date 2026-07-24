package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestCanUseAsCarryScriptRejectsRegularNodePluginWithoutCarryMeta(t *testing.T) {
	fn := &common.Function{
		UUID:  "script.js",
		Type:  NODE,
		Rules: []string{"^hello$"},
	}
	if canUseAsCarryScript(fn) {
		t.Fatal("regular Node plugin without @carry should not be available as carry script")
	}
}

func TestCanUseAsCarryScriptAllowsCarryNodePlugin(t *testing.T) {
	fn := &common.Function{
		UUID:  "script.js",
		Type:  NODE,
		Carry: true,
		Rules: []string{"^hello$"},
	}
	if !canUseAsCarryScript(fn) {
		t.Fatal("Node plugin with @carry should be available as carry script")
	}
}

func TestCanUseAsCarryScriptRejectsLongRunningPlugin(t *testing.T) {
	fn := &common.Function{
		UUID:    "web.js",
		Type:    NODE,
		OnStart: true,
		Web:     true,
	}
	if canUseAsCarryScript(fn) {
		t.Fatal("long-running Node plugin should not be available as carry script")
	}
}

func TestGetAdapterBotsIDReturnsAllWhenPlatformEmpty(t *testing.T) {
	BotsLocker.Lock()
	original := Bots
	Bots = map[Bot]*Factory{
		{"qq", "10001"}:       {},
		{"telegram", "20002"}: {},
	}
	BotsLocker.Unlock()
	defer func() {
		BotsLocker.Lock()
		Bots = original
		BotsLocker.Unlock()
	}()

	all := GetAdapterBotsID("")
	if !Contains(all, "10001") || !Contains(all, "20002") {
		t.Fatalf("GetAdapterBotsID(\"\") = %#v, want all bot IDs", all)
	}
	qq := GetAdapterBotsID("qq")
	if len(qq) != 1 || qq[0] != "10001" {
		t.Fatalf("GetAdapterBotsID(\"qq\") = %#v, want [10001]", qq)
	}
}

func TestPluginParseCarryMetaWithoutValue(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title 搬运处理
 * @carry
 */
module.exports = async sender => sender.reply("ok");
`, "carry.js")
	if !fn.Carry {
		t.Fatal("@carry without value should enable carry script")
	}
}

func TestPluginParseCarryFalse(t *testing.T) {
	fn, _ := pluginParse(`/**
 * @title 非搬运处理
 * @carry false
 */
module.exports = async sender => sender.reply("ok");
`, "carry.js")
	if fn.Carry {
		t.Fatal("@carry false should not enable carry script")
	}
}
