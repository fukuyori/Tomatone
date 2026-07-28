//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

func newMPVIPCPath() string {
	return fmt.Sprintf(`\\.\pipe\tomatone-mpv-%d-%d`, os.Getpid(), time.Now().UnixNano())
}

func connectMPVIPC(path string) (io.ReadWriteCloser, error) {
	var lastErr error
	for range 80 {
		connection, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	return nil, fmt.Errorf("mpv IPCへ接続できません: %w", lastErr)
}

func cleanupMPVIPC(_ string) {}
