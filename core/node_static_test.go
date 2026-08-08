package core

import (
	"testing"
)

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
