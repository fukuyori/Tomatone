//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	enableEchoInput     = 0x0004
	enableLineInput     = 0x0002
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
	keyEvent            = 0x0001
	virtualKeyEnter     = 0x0d
	virtualKeyEscape    = 0x1b
	virtualKeyUp        = 0x26
	virtualKeyDown      = 0x28
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	getConsoleModeProc   = kernel32.NewProc("GetConsoleMode")
	setConsoleModeProc   = kernel32.NewProc("SetConsoleMode")
	readConsoleInputProc = kernel32.NewProc("ReadConsoleInputW")
)

type consoleInputRecord struct {
	EventType uint16
	_         uint16
	Event     [4]uint32
}

type consoleKeyEvent struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

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

func readImmediateKey(file *os.File) (string, error) {
	for {
		var record consoleInputRecord
		var count uint32
		ok, _, callErr := readConsoleInputProc.Call(
			file.Fd(),
			uintptr(unsafe.Pointer(&record)),
			1,
			uintptr(unsafe.Pointer(&count)),
		)
		if ok == 0 {
			return "", fmt.Errorf("キー入力を読み取れません: %w", callErr)
		}
		if count == 0 || record.EventType != keyEvent {
			continue
		}

		event := (*consoleKeyEvent)(unsafe.Pointer(&record.Event[0]))
		if event.KeyDown == 0 {
			continue
		}
		switch event.VirtualKeyCode {
		case virtualKeyUp:
			return sourceKeyUp, nil
		case virtualKeyDown:
			return sourceKeyDown, nil
		case virtualKeyEscape:
			return sourceKeyEscape, nil
		case virtualKeyEnter:
			return sourceKeyEnter, nil
		}
		if event.UnicodeChar != 0 {
			return strings.ToLower(string(rune(event.UnicodeChar))), nil
		}
	}
}
