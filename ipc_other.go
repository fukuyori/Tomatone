//go:build !windows

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

func newMPVIPCPath() string {
	return filepath.Join(os.TempDir(),
		fmt.Sprintf("tomatone-mpv-%d-%d.sock", os.Getpid(), time.Now().UnixNano()))
}

func connectMPVIPC(path string) (io.ReadWriteCloser, error) {
	var lastErr error
	for range 80 {
		connection, err := net.Dial("unix", path)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	return nil, fmt.Errorf("mpv IPCへ接続できません: %w", lastErr)
}

func cleanupMPVIPC(path string) {
	_ = os.Remove(path)
}
