package core

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestAdminNormalUserLifecycle(t *testing.T) {
	username := fmt.Sprintf("admin_test_%d", time.Now().UnixNano())
	user, err := createNormalUser(username, "initial-password", "初始昵称")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deleteNormalUser(username) }()

	bindings, err := replaceNormalUserBindings(username, "12345678", "-123456789", []string{" openid-a ", "openid-b", "openid-a"})
	if err != nil {
		t.Fatal(err)
	}
	if bindings.QQ != "12345678" || bindings.Telegram != "-123456789" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
	if len(bindings.SmallcatOpenIDs) != 2 || bindings.SmallcatOpenID != "openid-a" {
		t.Fatalf("unexpected openids: %+v", bindings.SmallcatOpenIDs)
	}

	disabled := true
	updated, updatedBindings, err := updateNormalUserByAdmin(adminNormalUserPayload{
		Username:        username,
		Password:        "updated-password",
		Nickname:        "更新昵称",
		Disabled:        &disabled,
		QQ:              "87654321",
		Telegram:        "987654321",
		SmallcatOpenIDs: []string{"openid-c", "openid-d"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "更新昵称" || !updated.Disabled {
		t.Fatalf("unexpected updated user: %+v", updated)
	}
	if bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("updated-password")) != nil {
		t.Fatal("password was not updated")
	}
	if updatedBindings.QQ != "87654321" || len(updatedBindings.SmallcatOpenIDs) != 2 {
		t.Fatalf("unexpected updated bindings: %+v", updatedBindings)
	}

	authorizationKey := pluginUserAuthorizationKey(user.ID, "test-plugin")
	if _, _, err := pluginUserAuthorizations.Set(authorizationKey, true); err != nil {
		t.Fatal(err)
	}
	if err := deleteNormalUser(username); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNormalUser(username); err == nil {
		t.Fatal("deleted user can still be loaded")
	}
	if raw := userBucket.GetString(normalUserBindingsStorageKey(username)); raw != "" {
		t.Fatalf("bindings were not deleted: %s", raw)
	}
	if raw := pluginUserAuthorizations.GetString(authorizationKey); raw != "" {
		t.Fatalf("plugin authorization was not deleted: %s", raw)
	}
}

func TestNormalizedReplacementBindingsValidation(t *testing.T) {
	if _, err := normalizedReplacementBindings("abc", "", nil); err == nil {
		t.Fatal("invalid QQ should be rejected")
	}
	if _, err := normalizedReplacementBindings("123456", "telegram-user", nil); err == nil {
		t.Fatal("invalid Telegram ID should be rejected")
	}
}
