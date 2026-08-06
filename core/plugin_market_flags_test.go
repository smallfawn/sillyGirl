package core

import (
	"encoding/json"
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestPluginMarketFlagsAreSerialized(t *testing.T) {
	payload, err := json.Marshal(&common.Function{
		Admin: true,
		Cron:  map[string]string{"task": "*/5 * * * *"},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["admin"] != true {
		t.Fatalf("admin = %#v; want true", decoded["admin"])
	}
	cron, ok := decoded["cron"].(map[string]interface{})
	if !ok || cron["task"] != "*/5 * * * *" {
		t.Fatalf("cron = %#v; want task schedule", decoded["cron"])
	}
}

func TestGithubPublicIndexReadsMarketFlags(t *testing.T) {
	records, err := parseGithubPublicFileIndex([]byte(`{
  "plugins/timer.js": {
    "title": "Timer",
    "path": "plugins/timer.js",
    "type": "node",
    "admin": true,
    "cron": "*/5 * * * *"
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || !records[0].Admin || records[0].Cron != "*/5 * * * *" {
		t.Fatalf("records = %#v; want admin timer metadata", records)
	}
}
