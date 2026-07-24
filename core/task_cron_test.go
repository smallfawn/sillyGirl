package core

import (
	"strings"
	"testing"
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
	script := "/**\n * @title Demo\n */\nconsole.log('ok');\n"
	updated := upsertPluginCronAnnotation(script, "0 * * * *")
	if !strings.Contains(updated, " * @cron 0 * * * *\n */") {
		t.Fatalf("cron line was not inserted into header:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "*/5 * * * *")
	if strings.Count(updated, "@cron") != 1 || !strings.Contains(updated, "@cron */5 * * * *") {
		t.Fatalf("cron line was not updated cleanly:\n%s", updated)
	}

	updated = upsertPluginCronAnnotation(updated, "")
	if strings.Contains(updated, "@cron") {
		t.Fatalf("cron line was not removed:\n%s", updated)
	}
}
