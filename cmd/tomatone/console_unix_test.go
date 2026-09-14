//go:build darwin || linux

package main

import (
	"os"
	"testing"
)

func TestEnableImmediateInputFallsBackForPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	restore, enabled, err := enableImmediateInput(reader)
	if err != nil {
		t.Fatalf("enableImmediateInput() error = %v", err)
	}
	defer restore()
	if enabled {
		t.Fatal("enableImmediateInput() enabled immediate input for a pipe")
	}
}

func TestReadImmediateKey(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "letter", input: "P", want: "p"},
		{name: "enter", input: "\r", want: sourceKeyEnter},
		{name: "escape", input: "\x1b", want: sourceKeyEscape},
		{name: "up", input: "\x1b[A", want: sourceKeyUp},
		{name: "down", input: "\x1b[B", want: sourceKeyDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			if _, err := writer.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}

			got, err := readImmediateKey(reader)
			if err != nil {
				t.Fatalf("readImmediateKey() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("readImmediateKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
