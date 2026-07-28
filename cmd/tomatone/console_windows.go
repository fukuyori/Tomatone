//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableEchoInput     = 0x0004
	enableLineInput     = 0x0002
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getConsoleModeProc = kernel32.NewProc("GetConsoleMode")
	setConsoleModeProc = kernel32.NewProc("SetConsoleMode")
)

func enableImmediateInput(file *os.File) (restore func(), enabled bool, err error) {
	handle := file.Fd()
	var original uint32
	ok, _, _ := getConsoleModeProc.Call(handle, uintptr(unsafe.Pointer(&original)))
	if ok == 0 {
		// Redirected stdin is not a console. The caller will use line input.
		return func() {}, false, nil
	}

	raw := original
	raw &^= enableEchoInput | enableLineInput | enableQuickEditMode
	raw |= enableExtendedFlags
	ok, _, callErr := setConsoleModeProc.Call(handle, uintptr(raw))
	if ok == 0 {
		return nil, false, fmt.Errorf("コンソールをCUI入力モードに変更できません: %w", callErr)
	}

	restore = func() {
		setConsoleModeProc.Call(handle, uintptr(original)) //nolint:errcheck
	}
	return restore, true, nil
}
