package main

import (
	"testing"
	"time"
)

func testRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Focus:                   2 * time.Second,
		ShortBreak:              time.Second,
		LongBreak:               3 * time.Second,
		FocusSessionsBeforeLong: 2,
		AutoStart:               true,
	}
}

func TestPomodoroTransitionsToShortAndLongBreak(t *testing.T) {
	timer := NewPomodoro(testRuntimeConfig())

	if !timer.Tick(2 * time.Second) {
		t.Fatal("focus completion should report a transition")
	}
	if timer.Phase() != PhaseShortBreak || timer.Completed() != 1 {
		t.Fatalf("after first focus: phase=%v completed=%d", timer.Phase(), timer.Completed())
	}

	timer.Tick(time.Second)
	if timer.Phase() != PhaseFocus || timer.SessionNumber() != 2 {
		t.Fatalf("after short break: phase=%v session=%d", timer.Phase(), timer.SessionNumber())
	}

	timer.Tick(2 * time.Second)
	if timer.Phase() != PhaseLongBreak || timer.Completed() != 2 {
		t.Fatalf("after second focus: phase=%v completed=%d", timer.Phase(), timer.Completed())
	}

	timer.Tick(3 * time.Second)
	if timer.Phase() != PhaseFocus || timer.SessionNumber() != 1 {
		t.Fatalf("after long break: phase=%v session=%d", timer.Phase(), timer.SessionNumber())
	}
}

func TestPomodoroPauseResetAndSkip(t *testing.T) {
	timer := NewPomodoro(testRuntimeConfig())
	timer.TogglePause()
	timer.Tick(time.Second)
	if timer.Remaining() != 2*time.Second {
		t.Fatalf("paused remaining = %v", timer.Remaining())
	}

	timer.TogglePause()
	timer.Tick(time.Second)
	timer.Reset()
	if timer.Remaining() != 2*time.Second {
		t.Fatalf("reset remaining = %v", timer.Remaining())
	}

	timer.Skip()
	if timer.Phase() != PhaseShortBreak || timer.Completed() != 0 {
		t.Fatalf("skip should not count focus: phase=%v completed=%d", timer.Phase(), timer.Completed())
	}
}

func TestPomodoroCarriesElapsedAcrossTransitions(t *testing.T) {
	timer := NewPomodoro(testRuntimeConfig())
	timer.Tick(2500 * time.Millisecond)
	if timer.Phase() != PhaseShortBreak || timer.Remaining() != 500*time.Millisecond {
		t.Fatalf("phase=%v remaining=%v", timer.Phase(), timer.Remaining())
	}
}

func TestParsePlayerStatus(t *testing.T) {
	player := NewAmbientPlayer(YouTubeConfig{
		URLs: []string{"https://youtu.be/test"},
	})
	player.generation = 1
	player.snapshot.Total = 1

	player.parseLine("TOMATONE_STATUS\tRainy Cafe\t62.5\t3600\tno\t61.5", 1)
	got := player.Snapshot()
	if got.State != PlayerPlaying || got.Title != "Rainy Cafe" {
		t.Fatalf("snapshot = %#v", got)
	}
	if got.Position != 62500*time.Millisecond || got.Duration != time.Hour {
		t.Fatalf("position=%v duration=%v", got.Position, got.Duration)
	}
	if got.Volume != 62 {
		t.Fatalf("volume=%d, want 62", got.Volume)
	}
}

func TestParseRadioICYTitle(t *testing.T) {
	player := NewAmbientPlayer(AmbienceConfig{
		URLs: []string{"https://radio.example.com/live.mp3"},
	})
	player.generation = 1

	player.parseLine("TOMATONE_STATUS\tStation Name\t12\t0\tno\t45\tArtist - Track", 1)
	got := player.Snapshot()
	if got.Title != "Artist - Track" {
		t.Fatalf("title = %q, want ICY title", got.Title)
	}
}
