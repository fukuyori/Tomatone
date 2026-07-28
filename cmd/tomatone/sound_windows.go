//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

var (
	winmm          = syscall.NewLazyDLL("winmm.dll")
	procPlaySoundW = winmm.NewProc("PlaySoundW")
	tempFileCache  sync.Map
)

const (
	SND_ASYNC     = 0x0001
	SND_NODEFAULT = 0x0002
	SND_FILENAME  = 0x00020000
)

func playAudioBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	tempPath := getOrCreateTempWavFile(data)
	if tempPath == "" {
		fmt.Print("\a")
		return
	}

	tempPtr, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		fmt.Print("\a")
		return
	}

	r, _, _ := procPlaySoundW.Call(
		uintptr(unsafe.Pointer(tempPtr)),
		0,
		uintptr(SND_ASYNC|SND_FILENAME|SND_NODEFAULT),
	)
	if r == 0 {
		fmt.Print("\a")
	}
}

func getOrCreateTempWavFile(data []byte) string {
	key := fmt.Sprintf("%p_%d", &data[0], len(data))
	if cached, ok := tempFileCache.Load(key); ok {
		return cached.(string)
	}

	fileName := fmt.Sprintf("tomatone_chime_%d.wav", len(data))
	filePath := filepath.Join(os.TempDir(), fileName)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return ""
	}

	tempFileCache.Store(key, filePath)
	return filePath
}
