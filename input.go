package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
)

func readCommands(ctx context.Context, input *os.File, commands chan<- string, immediate bool) {
	defer close(commands)
	if immediate {
		readImmediateCommands(ctx, input, commands)
		return
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		if !sendCommand(ctx, commands, scanner.Text()) {
			return
		}
	}
}

func readImmediateCommands(ctx context.Context, input io.Reader, commands chan<- string) {
	buffer := make([]byte, 1)
	for {
		n, err := input.Read(buffer)
		if err != nil {
			return
		}
		if n != 1 {
			continue
		}
		command := strings.ToLower(string(buffer[0]))
		switch command {
		case "p", "s", "r", "n", "q", "+", "=", "-", "_", " ", "c":
			if !sendCommand(ctx, commands, command) {
				return
			}
		}
	}
}

func sendCommand(ctx context.Context, commands chan<- string, command string) bool {
	select {
	case commands <- command:
		return true
	case <-ctx.Done():
		return false
	}
}
