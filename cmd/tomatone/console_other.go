//go:build !windows

package main

import "os"

func enableImmediateInput(_ *os.File) (restore func(), enabled bool, err error) {
	return func() {}, false, nil
}
