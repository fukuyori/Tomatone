package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	cfg := defaultConfig()
	cfg.Timer.Focus = "45m"
	cfg.Ambience.URLs = []string{
		"https://www.youtube.com/watch?v=abc",
		"https://youtu.be/def",
		"https://radio.example.com/live.mp3",
	}

	got, err := validateConfig(cfg)
	if err != nil {
		t.Fatalf("validateConfig() error = %v", err)
	}
	if got.Focus != 45*time.Minute {
		t.Fatalf("Focus = %v, want 45m", got.Focus)
	}
	if len(got.Ambience.URLs) != 3 {
		t.Fatalf("URLs = %d, want 3", len(got.Ambience.URLs))
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{
			name: "duration",
			change: func(cfg *Config) {
				cfg.Timer.Focus = "25"
			},
			want: "timer.focus",
		},
		{
			name: "volume",
			change: func(cfg *Config) {
				cfg.Ambience.Volume = 101
			},
			want: "ambience.volume",
		},
		{
			name: "url",
			change: func(cfg *Config) {
				cfg.Ambience.URLs = []string{"file:///music.mp3"}
			},
			want: "HTTP(S)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			tt.change(&cfg)
			_, err := validateConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestWriteAndLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg := defaultConfig()
	cfg.Ambience.URLs = []string{"https://radio.example.com/live.m3u8"}
	if err := writeConfig(path, cfg, false); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config was not created: %v", err)
	}

	loaded, runtimeCfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if loaded.Timer.Focus != "25m" || len(runtimeCfg.Ambience.URLs) != 1 {
		t.Fatalf("unexpected loaded config: %#v", loaded)
	}
}

func TestLoadLegacyYouTubeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := `{
		"timer": {
			"focus": "25m",
			"short_break": "5m",
			"long_break": "15m",
			"focus_sessions_before_long_break": 4,
			"auto_start": true
		},
		"chime": {"volume": 80},
		"youtube": {
			"urls": ["https://youtu.be/legacy"],
			"shuffle": false,
			"volume": 45,
			"player_command": "mpv"
		}
	}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	_, runtimeCfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if len(runtimeCfg.Ambience.URLs) != 1 ||
		runtimeCfg.Ambience.URLs[0] != "https://youtu.be/legacy" {
		t.Fatalf("legacy URLs = %#v", runtimeCfg.Ambience.URLs)
	}
}

func TestConfigRejectsAmbienceAndLegacyYouTubeTogether(t *testing.T) {
	cfg := defaultConfig()
	legacy := *cfg.Ambience
	cfg.YouTube = &legacy
	_, err := validateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "同時に指定") {
		t.Fatalf("error = %v, want conflicting config error", err)
	}
}

func TestResolveNTSSlowFocusPage(t *testing.T) {
	const page = "https://www.nts.live/infinite-mixtapes/slow-focus"
	const stream = "https://stream-mixtape-geo.ntslive.net/mixtape?client=direct"
	if got := resolveMediaURL(page); got != stream {
		t.Fatalf("resolveMediaURL() = %q, want %q", got, stream)
	}
}

func TestResolveURLs(t *testing.T) {
	cfg := YouTubeConfig{
		URLs:      []string{"https://www.youtube.com/watch?v=default"},
		FocusURLs: []string{"https://www.youtube.com/watch?v=focus"},
		BreakURLs: []string{"https://www.youtube.com/watch?v=break"},
		TimeRules: []TimeRule{
			{
				Name:      "Night",
				Start:     "22:00",
				End:       "06:00",
				FocusURLs: []string{"https://www.youtube.com/watch?v=night_focus"},
				BreakURLs: []string{"https://www.youtube.com/watch?v=night_break"},
			},
		},
	}

	dayTime := time.Date(2026, 7, 27, 14, 0, 0, 0, time.Local)
	nightTime := time.Date(2026, 7, 27, 23, 0, 0, 0, time.Local)

	// Daytime Focus
	got := ResolveURLs(cfg, PhaseFocus, dayTime)
	if len(got) != 1 || got[0] != "https://www.youtube.com/watch?v=focus" {
		t.Fatalf("Daytime Focus = %v, want focus URL", got)
	}

	// Daytime Break
	got = ResolveURLs(cfg, PhaseShortBreak, dayTime)
	if len(got) != 1 || got[0] != "https://www.youtube.com/watch?v=break" {
		t.Fatalf("Daytime Break = %v, want break URL", got)
	}

	// Nighttime Focus
	got = ResolveURLs(cfg, PhaseFocus, nightTime)
	if len(got) != 1 || got[0] != "https://www.youtube.com/watch?v=night_focus" {
		t.Fatalf("Nighttime Focus = %v, want night_focus URL", got)
	}

	// Nighttime Break
	got = ResolveURLs(cfg, PhaseShortBreak, nightTime)
	if len(got) != 1 || got[0] != "https://www.youtube.com/watch?v=night_break" {
		t.Fatalf("Nighttime Break = %v, want night_break URL", got)
	}
}
