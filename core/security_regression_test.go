package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAesDecryptRejectsMalformedCiphertext(t *testing.T) {
	for _, ciphertext := range [][]byte{nil, {1}, make([]byte, 16)} {
		if _, err := AesDecrypt(ciphertext, PwdKey); err == nil {
			t.Fatalf("malformed ciphertext of length %d was accepted", len(ciphertext))
		}
	}

	plaintext := []byte("round-trip")
	ciphertext, err := AesEncrypt(plaintext, PwdKey)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := AesDecrypt(ciphertext, PwdKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatalf("round trip mismatch: got %q", decoded)
	}

	encoded, err := EncryptByAes(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "v2:") {
		t.Fatalf("new ciphertext does not use authenticated format: %q", encoded)
	}
	decoded, err = DecryptByAes(encoded)
	if err != nil || !bytes.Equal(decoded, plaintext) {
		t.Fatalf("authenticated round trip failed: decoded=%q err=%v", decoded, err)
	}
	tamperedPayload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "v2:"))
	if err != nil {
		t.Fatal(err)
	}
	tamperedPayload[len(tamperedPayload)-1] ^= 1
	tampered := "v2:" + base64.StdEncoding.EncodeToString(tamperedPayload)
	if _, err := DecryptByAes(tampered); err == nil {
		t.Fatal("tampered authenticated ciphertext was accepted")
	}

	legacy := base64.StdEncoding.EncodeToString(ciphertext)
	decoded, err = DecryptByAes(legacy)
	if err != nil || !bytes.Equal(decoded, plaintext) {
		t.Fatalf("legacy ciphertext compatibility failed: decoded=%q err=%v", decoded, err)
	}
}

func TestUserAnnouncementHTMLIsSanitizedBeforeVHTML(t *testing.T) {
	source, err := os.ReadFile("../frontend/src/User.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "return sanitizeAnnouncementHTML(html)") {
		t.Fatal("announcement HTML does not pass through the sanitizer")
	}
	if strings.Contains(text, "if (mode === 'html') return String(content || '')") {
		t.Fatal("raw announcement HTML is still returned to v-html")
	}
}

func TestLoginAttemptLimitBlocksAccountSprayingFromOneIP(t *testing.T) {
	loginAttemptLock.Lock()
	original, originalPrune := loginAttempts, loginAttemptLastPrune
	loginAttempts = map[string]*loginAttemptState{}
	loginAttemptLastPrune = time.Time{}
	loginAttemptLock.Unlock()
	t.Cleanup(func() {
		loginAttemptLock.Lock()
		loginAttempts, loginAttemptLastPrune = original, originalPrune
		loginAttemptLock.Unlock()
	})

	gin.SetMode(gin.TestMode)
	for i := 0; i < maxLoginAttempts; i++ {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/api/user/login", nil)
		ctx.Request.RemoteAddr = "203.0.113.25:4321"
		recordFailedLoginAttempt(ctx, fmt.Sprintf("user:random-%d", i))
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/api/user/login", nil)
	ctx.Request.RemoteAddr = "203.0.113.25:9999"
	if !loginAttemptBlocked(ctx, "user:another-account") {
		t.Fatal("per-IP limiter allowed username spraying")
	}
}

func TestVerifyAdminPasswordMigratesLegacyCiphertext(t *testing.T) {
	original := password
	t.Cleanup(func() {
		password = original
		_, _, _ = sillyGirl.Set("password", original)
	})

	raw := "legacy-password"
	ciphertext, err := AesEncrypt([]byte(raw), PwdKey)
	if err != nil {
		t.Fatal(err)
	}
	password = base64.StdEncoding.EncodeToString(ciphertext)
	if !verifyAdminPassword(raw) {
		t.Fatal("legacy encrypted admin password was rejected")
	}
	if !isAdminPasswordHash(password) {
		t.Fatalf("legacy password was not migrated to bcrypt: %q", password)
	}
}

func TestCopyWithLimit(t *testing.T) {
	var target bytes.Buffer
	written, err := copyWithLimit(&target, strings.NewReader("1234"), 4)
	if err != nil || written != 4 || target.String() != "1234" {
		t.Fatalf("bounded copy failed: written=%d err=%v body=%q", written, err, target.String())
	}

	target.Reset()
	written, err = copyWithLimit(&target, strings.NewReader("12345"), 4)
	if err == nil || written != 5 {
		t.Fatalf("oversized copy was accepted: written=%d err=%v", written, err)
	}
}

func TestSafeExtractPathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", "sub/../../outside", "/absolute", `C:\\absolute`} {
		if _, err := safeExtractPath(root, name); err == nil {
			t.Fatalf("unsafe archive path was accepted: %q", name)
		}
	}
	if _, err := safeExtractPath(root, "sub/file"); err != nil {
		t.Fatalf("safe archive path was rejected: %v", err)
	}
}

func TestStringsRandomBoundsAndAlphabet(t *testing.T) {
	if got := mystr.Random(-1, "abc"); got != "" {
		t.Fatalf("negative length returned %q", got)
	}
	got := mystr.Random(64, "ab")
	if len(got) != 64 || strings.Trim(got, "ab") != "" {
		t.Fatalf("unexpected random output %q", got)
	}
}

func TestDecodePluginUserFormJSONRejectsOversizeAndTrailingValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{
		`{"uuid":"one"} {"uuid":"two"}`,
		`{"value":"` + strings.Repeat("x", maxPluginUserFormBodyBytes) + `"}`,
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("PUT", "/api/user/plugin/form", strings.NewReader(body))
		var payload map[string]interface{}
		if err := decodePluginUserFormJSON(ctx, &payload); err == nil {
			t.Fatalf("invalid request body was accepted")
		}
	}
}

func TestPluginUserRecordKeyHasNoSeparatorCollision(t *testing.T) {
	a := pluginUserRecordKey(map[string]interface{}{"a": "x\x1fy", "b": "z"}, []string{"a", "b"})
	b := pluginUserRecordKey(map[string]interface{}{"a": "x", "b": "y\x1fz"}, []string{"a", "b"})
	if a == b {
		t.Fatal("keyBy values produced a collision")
	}
}
