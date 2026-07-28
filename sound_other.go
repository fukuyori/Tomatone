//go:build !windows

package main

import "fmt"

func playAudioBytes(data []byte) {
	fmt.Print("\a")
}
