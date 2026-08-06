package core

import (
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestPluginExecutionEnabledUsesFormEnable(t *testing.T) {
	uuid := "test-plugin-execution-enable"
	defer deletePluginConfig(uuid, true)
	_, _, _ = SetBucketKeyValue(pluginConfigSchemas, uuid, map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{"type": "boolean", "default": true},
		},
	})
	f := &common.Function{UUID: uuid}
	if !pluginExecutionEnabled(f) {
		t.Fatal("form default true unexpectedly disabled the plugin")
	}
	_, _, _ = SetBucketKeyValue(pluginConfigValues, uuid, map[string]interface{}{"enable": false})
	if pluginExecutionEnabled(f) {
		t.Fatal("saved enable=false did not disable the plugin")
	}
	_, _, _ = SetBucketKeyValue(pluginConfigValues, uuid, map[string]interface{}{"enable": true})
	if !pluginExecutionEnabled(f) {
		t.Fatal("saved enable=true did not enable the plugin")
	}
}

func TestPluginExecutionEnabledWithoutEnableField(t *testing.T) {
	if !pluginExecutionEnabled(&common.Function{UUID: "plugin-without-enable-field"}) {
		t.Fatal("plugin without reserved enable field should stay enabled")
	}
	if pluginExecutionEnabled(&common.Function{Disable: true}) {
		t.Fatal("hard-disabled plugin unexpectedly enabled")
	}
}
