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
		"● 再生中",
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

func TestUIMessageExpires(t *testing.T) {
	var output strings.Builder
	ui := NewUI(&output, "config.json", true)
	ui.SetMessage("次のアンビエンスへ切り替えています")

	if got := ui.currentMessage(time.Now()); got == "" {
		t.Fatal("new message should be visible")
	}

	got := ui.currentMessage(time.Now().Add(uiMessageDuration))
	if got != "" {
		t.Fatalf("expired message = %q, want empty", got)
	}
	if ui.message != "" || !ui.messageEnd.IsZero() {
		t.Fatal("expired message state should be cleared")
	}
}

func TestClearingUIMessageResetsExpiration(t *testing.T) {
	var output strings.Builder
	ui := NewUI(&output, "config.json", true)
	ui.SetMessage("message")
	ui.SetMessage("")

	if got := ui.currentMessage(time.Now()); got != "" {
		t.Fatalf("cleared message = %q, want empty", got)
	}
	if !ui.messageEnd.IsZero() {
		t.Fatal("clearing a message should reset its expiration")
	}
}

func TestPlayerStateReplacesExpiredOperationMessageOnSameLine(t *testing.T) {
	cfg := testRuntimeConfig()
	timer := NewPomodoro(cfg)
	player := PlayerSnapshot{
		State:  PlayerPlaying,
		Title:  "Rainy Cafe",
		Total:  2,
		Volume: 45,
	}

	var switchingOutput strings.Builder
	ui := NewUI(&switchingOutput, "config.json", true)
	ui.SetMessage("次のアンビエンスに切り替えています")
	ui.Render(timer, player)

	switchingLine := lineContaining(switchingOutput.String(), "次のアンビエンスに切り替えています")
	if switchingLine < 0 {
		t.Fatal("switching message is not rendered")
	}
	if strings.Contains(switchingOutput.String(), "● 再生中") {
		t.Fatal("player state should be hidden while the switching message is visible")
	}

	var playingOutput strings.Builder
	ui.out = &playingOutput
	ui.messageEnd = time.Now().Add(-time.Second)
	ui.Render(timer, player)

	playingLine := lineContaining(playingOutput.String(), "● 再生中")
	if playingLine < 0 {
		t.Fatal("player state is not rendered after the switching message expires")
	}
	if playingLine != switchingLine {
		t.Fatalf("player state line = %d, switching message line = %d", playingLine, switchingLine)
	}
	if strings.Contains(playingOutput.String(), "次のアンビエンスに切り替えています") {
		t.Fatal("expired switching message should not be rendered")
	}
}

func lineContaining(output, value string) int {
	for index, line := range strings.Split(output, "\n") {
		if strings.Contains(line, value) {
			return index
		}
	}
	return -1
}
