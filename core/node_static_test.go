package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSafeStaticFilePathAllowsChildPath(t *testing.T) {
	root := t.TempDir()
	got, err := safeStaticFilePath(root, "assets/app.js")
	if err != nil {
		t.Fatalf("safeStaticFilePath returned error: %v", err)
	}
	want := filepath.Join(root, "assets", "app.js")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("safeStaticFilePath = %q, want %q", got, want)
	}
}

func TestSafeStaticFilePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../secret.txt",
		"..\\secret.txt",
		"assets/../../secret.txt",
		"/etc/passwd",
		"C:\\Windows\\win.ini",
		"assets/\x00/app.js",
	}
	for _, item := range cases {
		if got, err := safeStaticFilePath(root, item); err == nil {
			t.Fatalf("safeStaticFilePath(%q) = %q, want error", item, got)
		}
	}
}

func TestSafeStaticFilePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if got, err := safeStaticFilePath(root, "linked/secret.txt"); err == nil {
		t.Fatalf("safeStaticFilePath(symlink escape) = %q, want error", got)
	}
}

func TestFindFileRejectsTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "public.txt"), []byte("public"), 0644); err != nil {
		t.Fatal(err)
	}
	uuid := strings.ReplaceAll(t.Name(), "/", "-")
	addStatic(uuid, root)
	defer remStatic(uuid)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "filename", Value: "..\\secret.txt"}}
	FindFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("FindFile traversal status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFindFileServesChildFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("console.log('ok');"), 0644); err != nil {
		t.Fatal(err)
	}
	uuid := strings.ReplaceAll(t.Name(), "/", "-")
	addStatic(uuid, root)
	defer remStatic(uuid)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/files/assets/app.js", nil)
	c.Params = gin.Params{{Key: "filename", Value: "assets/app.js"}}
	FindFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("FindFile child file status = %d, want %d", w.Code, http.StatusOK)
	}
	if body := w.Body.String(); body != "console.log('ok');" {
		t.Fatalf("FindFile body = %q", body)
	}
}

func TestFindFileRouteServesNestedAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("console.log('route');"), 0644); err != nil {
		t.Fatal(err)
	}
	uuid := strings.ReplaceAll(t.Name(), "/", "-")
	addStatic(uuid, root)
	defer remStatic(uuid)

	router := gin.New()
	router.GET("/api/files/*filename", FindFile)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files/assets/app.js", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("nested static route status = %d, want %d", w.Code, http.StatusOK)
	}
	if body := w.Body.String(); body != "console.log('route');" {
		t.Fatalf("nested static route body = %q", body)
	}
}

func TestSafeEmbeddedFileNameRejectsTraversal(t *testing.T) {
	cases := []string{
		"admin/assets/../index.html",
		"admin\\assets\\..\\index.html",
		"../VERSION",
		"C:\\Windows\\win.ini",
		"admin/assets/app.js\x00",
	}
	for _, item := range cases {
		if got, err := safeEmbeddedFileName(item); err == nil {
			t.Fatalf("safeEmbeddedFileName(%q) = %q, want error", item, got)
		}
	}
}

func TestSafeEmbeddedFileNameAllowsAdminAssets(t *testing.T) {
	got, err := safeEmbeddedFileName("/admin/assets/app.js")
	if err != nil {
		t.Fatalf("safeEmbeddedFileName returned error: %v", err)
	}
	if got != "admin/assets/app.js" {
		t.Fatalf("safeEmbeddedFileName = %q, want admin/assets/app.js", got)
	}
}
