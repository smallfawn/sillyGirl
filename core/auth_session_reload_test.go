package core

import (
	"strings"
	"testing"
	"time"

	"github.com/smallfawn/sillyGirl/utils"
)

func removeCachedAdminSessionForTest(token string) {
	authsLock.Lock()
	defer authsLock.Unlock()
	filtered := auths[:0]
	for _, auth := range auths {
		if auth == nil || auth.Token != token {
			filtered = append(filtered, auth)
		}
	}
	auths = filtered
}

func persistedAdminSessionForTest(t *testing.T, age time.Duration) *Auth {
	t.Helper()
	auth := &Auth{
		Token:     "auth-reload-test-" + utils.GenUUID(),
		CreatedAt: int(time.Now().Add(-age).Unix()),
	}
	if err := authBucket.Create(auth); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		removeCachedAdminSessionForTest(auth.Token)
		_, _, _ = authBucket.Set(auth.ID, "")
	})
	return auth
}

func TestAdminSessionReloadsFromStorageAfterCacheMiss(t *testing.T) {
	// This session is older than the previous one-day startup cutoff but still
	// inside the three-day JWT lifetime.
	auth := persistedAdminSessionForTest(t, 48*time.Hour)
	removeCachedAdminSessionForTest(auth.Token)

	got, err := checkSessionToken(auth.Token)
	if err != nil {
		t.Fatalf("persisted session was rejected after cache miss: %v", err)
	}
	if got.Token != auth.Token || cachedAdminSession(auth.Token) == nil {
		t.Fatalf("session was not restored into cache: %#v", got)
	}
}

func TestAdminJWTAuthReloadsPersistedSessionAfterRestart(t *testing.T) {
	auth := persistedAdminSessionForTest(t, 48*time.Hour)
	removeCachedAdminSessionForTest(auth.Token)
	username := strings.TrimSpace(sillyGirl.GetString("name"))
	if username == "" {
		username = "傻妞"
	}
	issuedAt := int64(auth.CreatedAt)
	token, err := signAdminJWT(adminJWTClaims{
		Sub: username,
		JTI: auth.Token,
		Iat: issuedAt,
		Exp: issuedAt + adminJWTExpireSeconds,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := CheckAuth(token)
	if err != nil {
		t.Fatalf("valid JWT was rejected after simulated restart: %v", err)
	}
	if got.Token != auth.Token {
		t.Fatalf("reloaded JWT session = %#v; want token %q", got, auth.Token)
	}
}

func TestAdminSessionCacheMissRejectsExpiredStoredSession(t *testing.T) {
	auth := persistedAdminSessionForTest(t, 4*24*time.Hour)
	removeCachedAdminSessionForTest(auth.Token)

	if _, err := checkSessionToken(auth.Token); err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("expired stored session error = %v", err)
	}
}

func TestAdminSessionMissingErrorIsExplicit(t *testing.T) {
	if _, err := checkSessionToken("missing-" + utils.GenUUID()); err == nil || !strings.Contains(err.Error(), "会话不存在") {
		t.Fatalf("missing session error = %v", err)
	}
}
