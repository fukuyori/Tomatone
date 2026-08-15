package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func resolvePlayerCommand(command string) (string, error) {
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", err
	}
	return preferDirectPlayerExecutable(command, resolved), nil
}

func preferDirectPlayerExecutable(command, resolved string) string {
	if runtime.GOOS != "windows" || filepath.Ext(command) != "" || !strings.EqualFold(filepath.Ext(resolved), ".com") {
		return resolved
	}
	executable := strings.TrimSuffix(resolved, filepath.Ext(resolved)) + ".exe"
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		return executable
	}
	return resolved
}
