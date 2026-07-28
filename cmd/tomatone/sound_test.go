package main

import (
	"testing"
)

func TestGenerateChimeWav(t *testing.T) {
	if len(wavChimeToBreak) < 44 {
		t.Fatalf("wavChimeToBreak length = %d, want > 44", len(wavChimeToBreak))
	}
	if len(wavChimeToFocus) < 44 {
		t.Fatalf("wavChimeToFocus length = %d, want > 44", len(wavChimeToFocus))
	}

	// Verify WAV header
	if string(wavChimeToBreak[:4]) != "RIFF" || string(wavChimeToBreak[8:12]) != "WAVE" {
		t.Errorf("invalid WAV header for wavChimeToBreak")
	}
	if string(wavChimeToFocus[:4]) != "RIFF" || string(wavChimeToFocus[8:12]) != "WAVE" {
		t.Errorf("invalid WAV header for wavChimeToFocus")
	}
}

func TestApplyVolume(t *testing.T) {
	scaled := applyVolume(wavChimeToBreak, 0.5)
	if len(scaled) != len(wavChimeToBreak) {
		t.Fatalf("len = %d, want %d", len(scaled), len(wavChimeToBreak))
	}
	if string(scaled[:4]) != "RIFF" {
		t.Errorf("invalid header after scaling")
	}
}
