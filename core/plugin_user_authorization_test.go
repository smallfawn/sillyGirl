package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/proto3/srpc"
	"github.com/smallfawn/sillyGirl/utils"
	"google.golang.org/grpc/metadata"
)

func TestPluginUserSmallcatAuthorizationRecords(t *testing.T) {
	originalFunctions := Functions
	t.Cleanup(func() { Functions = originalFunctions })

	plugin := &common.Function{UUID: "authorization-plugin", Title: "授权插件", Type: NODE, Open: true, UsesSmallCat: true}
	Functions = []*common.Function{plugin}
	user := &normalUser{
		ID:        "authorization-user-id",
		Username:  "authorization-user",
		Nickname:  "授权用户",
		CreatedAt: 1,
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _, _ = userBucket.Set(normalUserStorageKey(user.Username), nil)
		_, _, _ = userBucket.Set(normalUserBindingsStorageKey(user.Username), nil)
		_, _, _ = pluginUserAuthorizations.Set(pluginUserAuthorizationKey(user.ID, plugin.UUID), nil)
	})
	bindings := normalUserBindings{
		QQ:              "10001",
		Telegram:        "20002",
		SmallcatOpenID:  "openid-a",
		SmallcatOpenIDs: []string{"openid-a", "openid-b"},
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(user.Username), utils.JsonMarshal(bindings)); err != nil {
		t.Fatal(err)
	}
	if err := setPluginUserSmallcatAuthorization(user, plugin.UUID, true); err != nil {
		t.Fatal(err)
	}

	rows := userOpenPluginRecords(user)
	if len(rows) != 1 || !rows[0].Authorized || rows[0].AuthorizationScope != pluginSmallcatReadScope {
		t.Fatalf("unexpected user plugin rows: %#v", rows)
	}
	if rows[0].SmallcatAccountCount != 2 {
		t.Fatalf("smallcat account count = %d, want 2", rows[0].SmallcatAccountCount)
	}

	records := pluginAuthorizedSmallcatRecords(plugin.UUID)
	if !records.Enforced || len(records.Users) != 1 || len(records.OpenIDs) != 2 {
		t.Fatalf("unexpected authorization records: %#v", records)
	}
	if records.Users[0].Username != user.Username {
		t.Fatalf("authorized username = %q", records.Users[0].Username)
	}

	value := pluginSmallcatRuntimeValue(plugin.UUID, pluginSmallcatRuntimeRecordKey)
	if !strings.HasPrefix(value, "o:") {
		t.Fatalf("runtime value is not an object value: %q", value)
	}
	decoded := pluginSmallcatRuntimeRecords{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(value, "o:")), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Enforced || len(decoded.OpenIDs) != 2 {
		t.Fatalf("decoded runtime records = %#v", decoded)
	}

	users := pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 {
		t.Fatalf("runtime user count = %d, want 1", len(users))
	}
	gotUser := users[0]
	if !gotUser.Authorized || gotUser.Disabled || gotUser.Bindings.QQ != "10001" || gotUser.Bindings.Telegram != "20002" {
		t.Fatalf("unexpected runtime user: %#v", gotUser)
	}
	if len(gotUser.Bindings.SmallcatOpenIDs) != 2 {
		t.Fatalf("runtime user openids = %#v", gotUser.Bindings.SmallcatOpenIDs)
	}
	userValue := pluginUserRuntimeValue(plugin.UUID, pluginUserRuntimeListKey)
	if !strings.HasPrefix(userValue, "o:[") {
		t.Fatalf("runtime user list is not an array value: %q", userValue)
	}
	runtimeID := "runtime-user-list-test"
	registerRuntimePlugin(runtimeID, plugin.UUID)
	t.Cleanup(func() { deleteSenderRegister(runtimeID) })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("runtime_id", runtimeID))
	response, err := (&SillyGirlService{}).BucketGet(ctx, &srpc.BucketKeyRequest{
		Name: pluginUserRuntimeBucket,
		Key:  pluginUserRuntimeListKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	grpcUsers := []pluginRuntimeUser{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(response.Value, "o:")), &grpcUsers); err != nil {
		t.Fatal(err)
	}
	if len(grpcUsers) != 1 || !grpcUsers[0].Authorized {
		t.Fatalf("gRPC runtime user view = %#v", grpcUsers)
	}
	service := &SillyGirlService{}
	if _, err := service.BucketSet(ctx, &srpc.BucketSetRequest{Name: pluginUserRuntimeBucket, Key: pluginUserRuntimeListKey, Value: "o:[]"}); err == nil {
		t.Fatal("runtime user view accepted a write")
	}
	keys, err := service.BucketKeys(ctx, &srpc.BucketRequest{Name: pluginUserRuntimeBucket})
	if err != nil || len(keys.Keys) != 1 || keys.Keys[0] != pluginUserRuntimeListKey {
		t.Fatalf("runtime user view keys = %#v, err = %v", keys, err)
	}
	length, err := service.BucketLen(ctx, &srpc.BucketRequest{Name: pluginUserRuntimeBucket})
	if err != nil || length.Length != 1 {
		t.Fatalf("runtime user view length = %#v, err = %v", length, err)
	}

	plugin.Open = false
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized || len(users[0].Bindings.SmallcatOpenIDs) != 0 {
		t.Fatalf("closed plugin retained effective authorization: %#v", users)
	}
	plugin.Open = true
	plugin.Disable = true
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized || len(users[0].Bindings.SmallcatOpenIDs) != 0 {
		t.Fatalf("disabled plugin retained effective authorization: %#v", users)
	}
	plugin.Disable = false
	plugin.UsesSmallCat = false
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized || len(users[0].Bindings.SmallcatOpenIDs) != 0 {
		t.Fatalf("non-smallcat plugin retained effective authorization: %#v", users)
	}
	plugin.UsesSmallCat = true
	user.Disabled = true
	if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
		t.Fatal(err)
	}
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized || len(users[0].Bindings.SmallcatOpenIDs) != 0 {
		t.Fatalf("disabled user retained effective authorization: %#v", users)
	}
	user.Disabled = false
	if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
		t.Fatal(err)
	}

	if err := setPluginUserSmallcatAuthorization(user, plugin.UUID, false); err != nil {
		t.Fatal(err)
	}
	if pluginUserSmallcatAuthorized(user.ID, plugin.UUID) {
		t.Fatal("revoked authorization is still enabled")
	}
	if raw := pluginUserAuthorizations.GetString(pluginUserAuthorizationKey(user.ID, plugin.UUID)); raw != "" {
		t.Fatalf("revoked authorization storage was not cleaned: %q", raw)
	}
	users = pluginRuntimeUsers(plugin.UUID)
	if len(users) != 1 || users[0].Authorized {
		t.Fatalf("revoked runtime authorization not reflected: %#v", users)
	}
	if len(users[0].Bindings.SmallcatOpenIDs) != 0 {
		t.Fatalf("revoked runtime user leaked smallcat openids: %#v", users[0])
	}
}

func TestPluginRuntimeUsersRejectUnknownPlugin(t *testing.T) {
	if users := pluginRuntimeUsers(""); len(users) != 0 {
		t.Fatalf("empty plugin runtime saw users: %#v", users)
	}
	if users := pluginRuntimeUsers("missing-plugin"); len(users) != 0 {
		t.Fatalf("unknown plugin runtime saw users: %#v", users)
	}
}

func TestPluginSmallcatAuthorizationPolicyLifecycle(t *testing.T) {
	originalFunctions := Functions
	t.Cleanup(func() { Functions = originalFunctions })
	Functions = []*common.Function{{UUID: "never-open-plugin", Type: NODE, Open: false}}

	records := pluginAuthorizedSmallcatRecords("never-open-plugin")
	if records.Enforced {
		t.Fatalf("never-open plugin should keep legacy account access: %#v", records)
	}

	Functions = []*common.Function{{UUID: "closed-managed-plugin", Type: NODE, Open: false}}
	if _, _, err := plugin_open.Set("closed-managed-plugin", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _, _ = plugin_open.Set("closed-managed-plugin", nil) })
	records = pluginAuthorizedSmallcatRecords("closed-managed-plugin")
	if !records.Enforced || len(records.OpenIDs) != 0 {
		t.Fatalf("closed managed plugin must receive an enforced empty list: %#v", records)
	}
}

func TestPluginIDBoundToRuntimeContext(t *testing.T) {
	registerRuntimePlugin("runtime-authorization-test", "plugin-authorization-test")
	t.Cleanup(func() { deleteSenderRegister("runtime-authorization-test") })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("runtime_id", "runtime-authorization-test"))
	if got := pluginIDFromRuntimeContext(ctx); got != "plugin-authorization-test" {
		t.Fatalf("pluginIDFromRuntimeContext = %q", got)
	}
}
