//go:build !windows && !darwin && !linux

package main

import (
	"bufio"
	"os"
	"strings"
)

func enableImmediateInput(_ *os.File) (restore func(), enabled bool, err error) {
	return func() {}, false, nil
}

func readImmediateKey(file *os.File) (string, error) {
	reader := bufio.NewReader(file)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}
