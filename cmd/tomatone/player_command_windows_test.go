//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferDirectPlayerExecutable(t *testing.T) {
	dir := t.TempDir()
	launcher := filepath.Join(dir, "mpv.com")
	executable := filepath.Join(dir, "mpv.exe")
	if err := os.WriteFile(launcher, []byte("launcher"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("player"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := preferDirectPlayerExecutable("mpv", launcher); got != executable {
		t.Fatalf("resolved player = %q, want %q", got, executable)
	}
	if got := preferDirectPlayerExecutable(launcher, launcher); got != launcher {
		t.Fatalf("explicit .com command = %q, want %q", got, launcher)
	}
}

func TestInstalledMPVResolvesToExecutable(t *testing.T) {
	resolved, err := resolvePlayerCommand("mpv")
	if err != nil {
		t.Skip("mpv is not installed")
	}
	if filepath.Ext(resolved) != ".exe" {
		t.Fatalf("mpv resolved to %q, want the direct .exe", resolved)
	}
}
