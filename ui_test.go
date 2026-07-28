package main

import (
	"strings"
	"testing"
	"time"
)

func TestPlaybackTime(t *testing.T) {
	tests := []struct {
		name     string
		snapshot PlayerSnapshot
		want     string
	}{
		{
			name: "known duration",
			snapshot: PlayerSnapshot{
				Position: 83 * time.Second,
				Duration: time.Hour,
			},
			want: "1:23 / 1:00:00",
		},
		{
			name: "live",
			snapshot: PlayerSnapshot{
				Position: 5 * time.Minute,
			},
			want: "5:00 / LIVE",
		},
		{
			name: "not started",
			want: "--:--",
		},
		{
			name: "live starting",
			snapshot: PlayerSnapshot{
				State: PlayerPlaying,
			},
			want: "LIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := playbackTime(tt.snapshot); got != tt.want {
				t.Fatalf("playbackTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCUIContainsTimerAndPlayerStatus(t *testing.T) {
	var output strings.Builder
	cfg := testRuntimeConfig()
	timer := NewPomodoro(cfg)
	player := PlayerSnapshot{
		State:    PlayerPlaying,
		Title:    "Rainy Cafe",
		URL:      "https://youtu.be/test",
		Index:    1,
		Total:    2,
		Volume:   45,
		Position: 90 * time.Second,
		Duration: time.Hour,
	}

	ui := NewUI(&output, "config.json", true)
	ui.Render(timer, player)
	got := output.String()
	for _, want := range []string{
		"00:02",
		"VOL 45%",
		"1:30 / 1:00:00",
		"TOMATONE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CUI output does not contain %q", want)
		}
	}
	if !strings.HasPrefix(got, "\x1b[2J\x1b[H") {
		t.Error("CUI frame must clear the screen before drawing")
	}
	if lines := strings.Count(got, "\n") + 1; lines > 13 {
		t.Errorf("CUI frame uses %d lines, want at most 13", lines)
	}
}

func TestTruncateUsesTerminalCellWidth(t *testing.T) {
	got := truncate("ゾーンに入るBGM アンビエント", 20)
	if displayWidth(got) > 20 {
		t.Fatalf("display width = %d, want <= 20: %q", displayWidth(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncate() = %q, want ellipsis", got)
	}
}

func TestAlignRowUsesWideCharacterWidth(t *testing.T) {
	got := alignRow("再生中", "音量 45%", 24)
	if displayWidth(got) != 24 {
		t.Fatalf("display width = %d, want 24: %q", displayWidth(got), got)
	}
}
