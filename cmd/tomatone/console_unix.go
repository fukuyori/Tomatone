//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func enableImmediateInput(file *os.File) (restore func(), enabled bool, err error) {
	original, err := readTermios(file.Fd())
	if errors.Is(err, syscall.ENOTTY) {
		// Redirected stdin is not a terminal. The caller will use line input.
		return func() {}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("コンソールの入力モードを取得できません: %w", err)
	}

	immediate := *original
	immediate.Lflag &^= syscall.ECHO | syscall.ICANON
	immediate.Cc[syscall.VMIN] = 1
	immediate.Cc[syscall.VTIME] = 0
	if err := writeTermios(file.Fd(), &immediate); err != nil {
		return nil, false, fmt.Errorf("コンソールをCUI入力モードに変更できません: %w", err)
	}

	restore = func() {
		writeTermios(file.Fd(), original) //nolint:errcheck
	}
	return restore, true, nil
}

func readTermios(fd uintptr) (*syscall.Termios, error) {
	var state syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlReadTermios,
		uintptr(unsafe.Pointer(&state)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return &state, nil
}

func writeTermios(fd uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlWriteTermios,
		uintptr(unsafe.Pointer(state)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func readImmediateKey(file *os.File) (string, error) {
	value, err := readConsoleByte(file)
	if err != nil {
		return "", err
	}
	if value == '\r' || value == '\n' {
		return sourceKeyEnter, nil
	}
	if value != '\x1b' {
		return strings.ToLower(string(value)), nil
	}

	// Arrow keys arrive as escape sequences. Wait briefly for the remaining
	// bytes so a standalone Escape key can still be handled immediately.
	sequence, err := readAvailableConsoleBytes(file, 2, 20*time.Millisecond)
	if err != nil {
		return "", err
	}
	if len(sequence) == 2 && sequence[0] == '[' {
		switch sequence[1] {
		case 'A':
			return sourceKeyUp, nil
		case 'B':
			return sourceKeyDown, nil
		}
	}
	return sourceKeyEscape, nil
}

func readConsoleByte(file *os.File) (byte, error) {
	var buffer [1]byte
	for {
		n, err := file.Read(buffer[:])
		if n == 1 {
			return buffer[0], nil
		}
		if err != nil {
			return 0, err
		}
	}
}

func readAvailableConsoleBytes(file *os.File, count int, timeout time.Duration) ([]byte, error) {
	result := make([]byte, 0, count)
	deadline := time.Now().Add(timeout)
	var buffer [1]byte
	for len(result) < count {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		ready, err := consoleInputReady(file.Fd(), remaining)
		if err != nil {
			return nil, err
		}
		if !ready {
			break
		}

		n, err := syscall.Read(int(file.Fd()), buffer[:])
		if n == 1 {
			result = append(result, buffer[0])
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func consoleInputReady(fd uintptr, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		var readSet syscall.FdSet
		bitsPerWord := int(unsafe.Sizeof(readSet.Bits[0])) * 8
		word := int(fd) / bitsPerWord
		if word >= len(readSet.Bits) {
			return false, fmt.Errorf("コンソールのファイル記述子が大きすぎます: %d", fd)
		}
		readSet.Bits[word] |= 1 << (uint(fd) % uint(bitsPerWord))
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, nil
		}
		wait := syscall.NsecToTimeval(remaining.Nanoseconds())
		ready, _, errno := syscall.Syscall6(
			syscall.SYS_SELECT,
			fd+1,
			uintptr(unsafe.Pointer(&readSet)),
			0,
			0,
			uintptr(unsafe.Pointer(&wait)),
			0,
		)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			return false, errno
		}
		return ready > 0, nil
	}
}
