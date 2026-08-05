package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestParseCronMetaValue(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"0 * * * *", "0 * * * *"},
		{"0 0 * * * *", "0 0 * * * *"},
		{"qq 0 * * * *", ""},
		{"telegram 0 0 * * * *", ""},
	}
	for _, tt := range tests {
		got := parseCronMetaValue(tt.value)
		if got != tt.want {
			t.Fatalf("parseCronMetaValue(%q) = %q; want %q", tt.value, got, tt.want)
		}
	}
}

func TestUpsertPluginCronAnnotation(t *testing.T) {
	script := "// [title: Demo]\n// [name: demo]\nconsole.log('ok');\n"
	updated := upsertPluginCronAnnotation(script, "0 * * * *", NODE)
	if !strings.Contains(updated, "// [cron: 0 * * * *]") {
		t.Fatalf("cron line was not inserted into header:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "*/5 * * * *", NODE)
	if strings.Count(updated, "[cron:") != 1 || !strings.Contains(updated, "// [cron: */5 * * * *]") {
		t.Fatalf("cron line was not updated cleanly:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "", NODE)
	if strings.Contains(updated, "[cron:") {
		t.Fatalf("cron line was not removed:\n%s", updated)
	}
}

func TestUpsertPluginCronAnnotationPython(t *testing.T) {
	script := "# [title: Demo]\n# [name: demo]\nprint('ok')\n"
	updated := upsertPluginCronAnnotation(script, "0 * * * *", PYTHON)
	if !strings.Contains(updated, "# [cron: 0 * * * *]") {
		t.Fatalf("cron line was not inserted into python header:\n%s", updated)
	}

	noHeader := "print('ok')\n"
	updated = upsertPluginCronAnnotation(noHeader, "0 * * * *", PYTHON)
	if !strings.HasPrefix(updated, "# [cron: 0 * * * *]\n\n") {
		t.Fatalf("python header was not created:\n%s", updated)
	}
}

func TestScriptCommandForPythonFunction(t *testing.T) {
	f := &common.Function{
		Type: PYTHON,
		Path: filepath.Join("plugins", "daily.py"),
	}
	if got := scriptCommandForFunction(f); got != "python daily.py" {
		t.Fatalf("scriptCommandForFunction() = %q; want %q", got, "python daily.py")
	}
	target, class := scriptTaskTarget("python daily.py")
	if target != "daily.py" || class != PYTHON {
		t.Fatalf("scriptTaskTarget() = (%q, %q); want (%q, %q)", target, class, "daily.py", PYTHON)
	}
}
