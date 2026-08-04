package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetDataHomeIsolatesTestProcess(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", "")
	want := filepath.Join(os.TempDir(), fmt.Sprintf("sillygirl-test-%d", os.Getpid()))
	if got := GetDataHome(); got != want {
		t.Fatalf("GetDataHome() = %q, want %q", got, want)
	}
}

func TestGetDataHomeHonorsExplicitPath(t *testing.T) {
	want := t.TempDir()
	t.Setenv("SILLYGIRL_DATA_PATH", want)
	if got := GetDataHome(); got != want {
		t.Fatalf("GetDataHome() = %q, want %q", got, want)
	}
}

func TestKillPeerIgnoresCurrentProcessPID(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	if err := os.WriteFile(GetPidFile(), []byte(fmt.Sprint(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := KillPeer(); err != nil {
		t.Fatalf("KillPeer returned an error for the current PID: %v", err)
	}
}
