package core

import "testing"

func TestPluginExecutionEnabledUsesStatusMetadataOnly(t *testing.T) {
	active, _ := pluginParse("// [status: true]\n", "test-plugin-status-active")
	if !pluginExecutionEnabled(active) {
		t.Fatal("[status: true] was not active")
	}

	inactive, _ := pluginParse("// [status: false]\n", "test-plugin-status-inactive")
	if pluginExecutionEnabled(inactive) {
		t.Fatal("[status: false] was still active")
	}
}

func TestPluginExecutionEnabledMetadataDefaults(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{name: "missing defaults enabled", script: "// [title: demo]\n", want: true},
		{name: "legacy true", script: "// [status: true]\n", want: true},
		{name: "legacy false", script: "// [status: false]\n", want: false},
		{name: "at true", script: "/**\n * @status yes\n */\n", want: true},
		{name: "at false", script: "/**\n * @status off\n */\n", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			function, _ := pluginParse(test.script, "plugin-status-metadata")
			if got := pluginExecutionEnabled(function); got != test.want {
				t.Fatalf("pluginExecutionEnabled() = %v; want %v", got, test.want)
			}
		})
	}

	if pluginExecutionEnabled(nil) {
		t.Fatal("nil plugin unexpectedly enabled")
	}
}

func TestInactiveStartupPluginIsNotReportedRunning(t *testing.T) {
	inactive, _ := pluginParse("// [status: false]\n// [web: true]\n", "inactive-web")
	if !inactive.OnStart || inactive.Running {
		t.Fatalf("inactive web plugin state = on_start:%v running:%v", inactive.OnStart, inactive.Running)
	}

	active, _ := pluginParse("// [status: true]\n// [web: true]\n", "active-web")
	if !active.OnStart || !active.Running {
		t.Fatalf("active web plugin state = on_start:%v running:%v", active.OnStart, active.Running)
	}
}
