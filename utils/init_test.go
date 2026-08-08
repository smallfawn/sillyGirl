package utils

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContains(t *testing.T) {
	values := []string{"alpha", "beta"}
	if !Contains(values, "missing", "beta") {
		t.Fatal("Contains did not find a matching candidate")
	}
	if Contains(values, "missing") {
		t.Fatal("Contains returned true for a missing candidate")
	}
	if Contains(nil, "alpha") {
		t.Fatal("Contains returned true for an empty source")
	}
}

func TestUniqueSkipsNonStringInterfaceValues(t *testing.T) {
	got := Unique([]interface{}{"alpha", 123, nil, "alpha", "beta"})
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("Unique returned %#v", got)
	}
}

func TestGetPublicIPFallsBackAndTrimsResponse(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/first" {
			http.Error(w, "upstream failure", http.StatusBadGateway)
			return
		}
		fmt.Fprintln(w, "203.0.113.7")
	}))
	defer server.Close()

	ip, err := getPublicIP(server.Client(), []string{server.URL + "/first", server.URL + "/second"})
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.7" || requests != 2 {
		t.Fatalf("getPublicIP returned ip=%q requests=%d", ip, requests)
	}
}

func TestGetPublicIPRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("1", int(publicIPResponseLimit)+1)))
	}))
	defer server.Close()

	if _, err := getPublicIP(server.Client(), []string{server.URL}); err == nil {
		t.Fatal("getPublicIP accepted an oversized response")
	}
}

func TestReadAllLimit(t *testing.T) {
	data, err := ReadAllLimit(bytes.NewBufferString("1234"), 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("ReadAllLimit exact limit returned data=%q err=%v", data, err)
	}
	if _, err := ReadAllLimit(bytes.NewBufferString("12345"), 4); err == nil {
		t.Fatal("ReadAllLimit accepted oversized input")
	}
}

func TestCopyFilePreservesContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(source, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(source, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "payload" {
		t.Fatalf("copied data=%q err=%v", data, err)
	}
}
