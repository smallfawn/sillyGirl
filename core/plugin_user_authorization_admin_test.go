package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/smallfawn/sillyGirl/core/common"
)

func TestAdminUserPluginAuthorizationLifecycle(t *testing.T) {
	originalFunctions := Functions
	t.Cleanup(func() { Functions = originalFunctions })

	plugin := &common.Function{
		UUID:         "admin-authorization-plugin",
		Title:        "授权插件",
		Type:         NODE,
		Open:         false,
		UsesSmallCat: true,
	}
	Functions = []*common.Function{plugin}

	username := fmt.Sprintf("apu%d", time.Now().UnixNano()%1_000_000_000)
	user, err := createNormalUser(username, "initial-password", "授权用户")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deleteNormalUser(username) })

	if err := setPluginUserSmallcatAuthorization(user, plugin.UUID, true); err != nil {
		t.Fatal(err)
	}

	rows := adminUserPluginAuthorizationsForUser(user.ID)
	if len(rows) != 1 || rows[0].UUID != plugin.UUID || !rows[0].Authorized {
		t.Fatalf("unexpected admin authorization rows: %#v", rows)
	}
	if rows[0].Title != "授权插件" || rows[0].Installed != true || rows[0].Open != false {
		t.Fatalf("unexpected plugin row metadata: %#v", rows[0])
	}

	list, err := listNormalUserPluginAuthorizations(username)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].UUID != plugin.UUID || !list[0].Authorized {
		t.Fatalf("unexpected plugin authorization list: %#v", list)
	}

	if err := setPluginUserSmallcatAuthorizationByUsername(username, plugin.UUID, false); err != nil {
		t.Fatal(err)
	}
	if pluginUserSmallcatAuthorized(user.ID, plugin.UUID) {
		t.Fatal("authorization was not removed")
	}

	if err := setPluginUserSmallcatAuthorizationByUsername(username, "missing-plugin", true); err == nil {
		t.Fatal("missing plugin should not be authorized")
	}
}
