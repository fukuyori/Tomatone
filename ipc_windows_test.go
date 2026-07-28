//go:build windows

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMPVIPCVolumeIntegration(t *testing.T) {
	if os.Getenv("TOMATONE_TEST_MPV") != "1" {
		t.Skip("set TOMATONE_TEST_MPV=1 to run the mpv integration test")
	}
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		mpvPath = filepath.Join(os.Getenv("USERPROFILE"), "scoop", "apps", "mpv", "current", "mpv.exe")
		if _, statErr := os.Stat(mpvPath); statErr != nil {
			t.Skip("mpv is not installed")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ipcPath := newMPVIPCPath()
	defer cleanupMPVIPC(ipcPath)
	cmd := exec.CommandContext(ctx, mpvPath,
		"--no-video",
		"--mute=yes",
		"--volume=45",
		"--input-ipc-server="+ipcPath,
		"av://lavfi:sine=frequency=440:duration=20",
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mpv: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	connection, err := connectMPVIPC(ipcPath)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	encoder := json.NewEncoder(connection)
	decoder := json.NewDecoder(connection)
	if err := encoder.Encode(map[string]any{
		"command":    []any{"add", "volume", 5},
		"request_id": 1,
	}); err != nil {
		t.Fatalf("send volume command: %v", err)
	}
	if err := encoder.Encode(map[string]any{
		"command":    []any{"get_property", "volume"},
		"request_id": 2,
	}); err != nil {
		t.Fatalf("query volume: %v", err)
	}

	type response struct {
		Data      float64 `json:"data"`
		Error     string  `json:"error"`
		RequestID int     `json:"request_id"`
	}
	for range 2 {
		var got response
		if err := decoder.Decode(&got); err != nil {
			t.Fatalf("read IPC response: %v", err)
		}
		if got.RequestID == 2 {
			if got.Error != "success" {
				t.Fatalf("get volume error: %s", got.Error)
			}
			if got.Data != 50 {
				t.Fatalf("volume = %v, want 50", got.Data)
			}
			return
		}
	}
	t.Fatal("volume response was not received")
}
