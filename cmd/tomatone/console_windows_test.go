//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestConsoleInputRecordLayout(t *testing.T) {
	var record consoleInputRecord
	if got := unsafe.Sizeof(record); got != 20 {
		t.Fatalf("consoleInputRecord size = %d, want 20", got)
	}
	if got := unsafe.Offsetof(record.Event); got != 4 {
		t.Fatalf("consoleInputRecord event offset = %d, want 4", got)
	}
	if got := unsafe.Sizeof(consoleKeyEvent{}); got != 16 {
		t.Fatalf("consoleKeyEvent size = %d, want 16", got)
	}
}
