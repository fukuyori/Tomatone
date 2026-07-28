package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Timer    TimerConfig     `json:"timer"`
	Chime    ChimeConfig     `json:"chime"`
	Ambience *AmbienceConfig `json:"ambience,omitempty"`
	YouTube  *AmbienceConfig `json:"youtube,omitempty"`
}

type TimerConfig struct {
	Focus                   string `json:"focus"`
	ShortBreak              string `json:"short_break"`
	LongBreak               string `json:"long_break"`
	FocusSessionsBeforeLong int    `json:"focus_sessions_before_long_break"`
	AutoStart               bool   `json:"auto_start"`
}

type ChimeConfig struct {
	Volume int `json:"volume"`
}

type AmbienceConfig struct {
	URLs          []string   `json:"urls"`
	FocusURLs     []string   `json:"focus_urls,omitempty"`
	BreakURLs     []string   `json:"break_urls,omitempty"`
	TimeRules     []TimeRule `json:"time_rules,omitempty"`
	Shuffle       bool       `json:"shuffle"`
	Volume        int        `json:"volume"`
	PlayerCommand string     `json:"player_command"`
}

// YouTubeConfig is retained as a source-compatible alias for older code.
type YouTubeConfig = AmbienceConfig

type TimeRule struct {
	Name      string   `json:"name,omitempty"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	URLs      []string `json:"urls,omitempty"`
	FocusURLs []string `json:"focus_urls,omitempty"`
	BreakURLs []string `json:"break_urls,omitempty"`
}

func (c AmbienceConfig) HasAnyURL() bool {
	if len(c.URLs) > 0 || len(c.FocusURLs) > 0 || len(c.BreakURLs) > 0 {
		return true
	}
	for _, rule := range c.TimeRules {
		if len(rule.URLs) > 0 || len(rule.FocusURLs) > 0 || len(rule.BreakURLs) > 0 {
			return true
		}
	}
	return false
}

type RuntimeConfig struct {
	Focus                   time.Duration
	ShortBreak              time.Duration
	LongBreak               time.Duration
	FocusSessionsBeforeLong int
	AutoStart               bool
	Chime                   ChimeConfig
	Ambience                AmbienceConfig
	YouTube                 AmbienceConfig
}

func defaultConfig() Config {
	return Config{
		Timer: TimerConfig{
			Focus:                   "25m",
			ShortBreak:              "5m",
			LongBreak:               "15m",
			FocusSessionsBeforeLong: 4,
			AutoStart:               true,
		},
		Chime: ChimeConfig{
			Volume: 80,
		},
		Ambience: &AmbienceConfig{
			URLs:          []string{},
			FocusURLs:     []string{},
			BreakURLs:     []string{},
			TimeRules:     []TimeRule{},
			Shuffle:       true,
			Volume:        45,
			PlayerCommand: "mpv",
		},
	}
}

func defaultConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("設定ディレクトリを取得できません: %w", err)
	}
	return filepath.Join(base, "Tomatone", "config.json"), nil
}

func ensureConfig(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("設定ファイルを確認できません: %w", err)
	}
	return true, writeConfig(path, defaultConfig(), false)
}

func writeConfig(path string, cfg Config, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("設定ファイルは既に存在します: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("設定ファイルを確認できません: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("設定ディレクトリを作成できません: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("設定をJSONに変換できません: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("設定ファイルを書き込めません: %w", err)
	}
	return nil
}

func loadConfig(path string) (Config, RuntimeConfig, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, RuntimeConfig{}, fmt.Errorf("設定ファイルを読めません: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, RuntimeConfig{}, fmt.Errorf("設定ファイルのJSONが不正です: %w", err)
	}
	runtimeCfg, err := validateConfig(cfg)
	if err != nil {
		return cfg, RuntimeConfig{}, err
	}
	return cfg, runtimeCfg, nil
}

func validateConfig(cfg Config) (RuntimeConfig, error) {
	focus, err := positiveDuration("timer.focus", cfg.Timer.Focus)
	if err != nil {
		return RuntimeConfig{}, err
	}
	shortBreak, err := positiveDuration("timer.short_break", cfg.Timer.ShortBreak)
	if err != nil {
		return RuntimeConfig{}, err
	}
	longBreak, err := positiveDuration("timer.long_break", cfg.Timer.LongBreak)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if cfg.Timer.FocusSessionsBeforeLong < 1 {
		return RuntimeConfig{}, errors.New("timer.focus_sessions_before_long_break は1以上にしてください")
	}
	if cfg.Chime.Volume < 0 || cfg.Chime.Volume > 100 {
		return RuntimeConfig{}, errors.New("chime.volume は0から100の範囲にしてください")
	}
	ambience, configKey, err := selectAmbienceConfig(cfg)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if ambience.Volume < 0 || ambience.Volume > 100 {
		return RuntimeConfig{}, fmt.Errorf("%s.volume は0から100の範囲にしてください", configKey)
	}
	if strings.TrimSpace(ambience.PlayerCommand) == "" {
		return RuntimeConfig{}, fmt.Errorf("%s.player_command を指定してください", configKey)
	}
	for i, raw := range ambience.URLs {
		if err := validateAudioURL(raw); err != nil {
			return RuntimeConfig{}, fmt.Errorf("%s.urls[%d]: %w", configKey, i, err)
		}
	}
	for i, raw := range ambience.FocusURLs {
		if err := validateAudioURL(raw); err != nil {
			return RuntimeConfig{}, fmt.Errorf("%s.focus_urls[%d]: %w", configKey, i, err)
		}
	}
	for i, raw := range ambience.BreakURLs {
		if err := validateAudioURL(raw); err != nil {
			return RuntimeConfig{}, fmt.Errorf("%s.break_urls[%d]: %w", configKey, i, err)
		}
	}
	for i, rule := range ambience.TimeRules {
		if _, _, err := parseHHMM(rule.Start); err != nil {
			return RuntimeConfig{}, fmt.Errorf("%s.time_rules[%d].start: %w", configKey, i, err)
		}
		if _, _, err := parseHHMM(rule.End); err != nil {
			return RuntimeConfig{}, fmt.Errorf("%s.time_rules[%d].end: %w", configKey, i, err)
		}
		for j, raw := range rule.URLs {
			if err := validateAudioURL(raw); err != nil {
				return RuntimeConfig{}, fmt.Errorf("%s.time_rules[%d].urls[%d]: %w", configKey, i, j, err)
			}
		}
		for j, raw := range rule.FocusURLs {
			if err := validateAudioURL(raw); err != nil {
				return RuntimeConfig{}, fmt.Errorf("%s.time_rules[%d].focus_urls[%d]: %w", configKey, i, j, err)
			}
		}
		for j, raw := range rule.BreakURLs {
			if err := validateAudioURL(raw); err != nil {
				return RuntimeConfig{}, fmt.Errorf("%s.time_rules[%d].break_urls[%d]: %w", configKey, i, j, err)
			}
		}
	}

	return RuntimeConfig{
		Focus:                   focus,
		ShortBreak:              shortBreak,
		LongBreak:               longBreak,
		FocusSessionsBeforeLong: cfg.Timer.FocusSessionsBeforeLong,
		AutoStart:               cfg.Timer.AutoStart,
		Chime:                   cfg.Chime,
		Ambience:                ambience,
		YouTube:                 ambience,
	}, nil
}

func selectAmbienceConfig(cfg Config) (AmbienceConfig, string, error) {
	if cfg.Ambience != nil && cfg.YouTube != nil {
		return AmbienceConfig{}, "", errors.New("ambience と旧形式の youtube は同時に指定できません")
	}
	if cfg.Ambience != nil {
		return *cfg.Ambience, "ambience", nil
	}
	if cfg.YouTube != nil {
		return *cfg.YouTube, "youtube", nil
	}
	return AmbienceConfig{}, "", errors.New("ambience 設定がありません")
}

func ResolveURLs(cfg AmbienceConfig, phase Phase, now time.Time) []string {
	isFocus := phase == PhaseFocus

	// 1. TimeRules check
	for _, rule := range cfg.TimeRules {
		if isInTimeRange(now, rule.Start, rule.End) {
			if isFocus && len(rule.FocusURLs) > 0 {
				return rule.FocusURLs
			}
			if !isFocus && len(rule.BreakURLs) > 0 {
				return rule.BreakURLs
			}
			if len(rule.URLs) > 0 {
				return rule.URLs
			}
		}
	}

	// 2. Phase check
	if isFocus && len(cfg.FocusURLs) > 0 {
		return cfg.FocusURLs
	}
	if !isFocus && len(cfg.BreakURLs) > 0 {
		return cfg.BreakURLs
	}

	// 3. Fallback
	return cfg.URLs
}

func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("時刻は HH:MM 形式で指定してください (例: 06:00)")
	}
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, 0, errors.New("時刻は HH:MM 形式で指定してください (例: 06:00)")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, errors.New("時刻の範囲が不正です (00:00 - 23:59)")
	}
	return h, m, nil
}

func isInTimeRange(now time.Time, startStr, endStr string) bool {
	sHour, sMin, err1 := parseHHMM(startStr)
	eHour, eMin, err2 := parseHHMM(endStr)
	if err1 != nil || err2 != nil {
		return false
	}
	currentMin := now.Hour()*60 + now.Minute()
	startMin := sHour*60 + sMin
	endMin := eHour*60 + eMin

	if startMin == endMin {
		return true
	}
	if startMin < endMin {
		return currentMin >= startMin && currentMin < endMin
	}
	return currentMin >= startMin || currentMin < endMin
}

func positiveDuration(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s は 25m や 1h30m の形式で指定してください: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s は0より大きくしてください", name)
	}
	return d, nil
}

func validateAudioURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return errors.New("有効なHTTP(S) URLではありません")
	}
	return nil
}

func isYouTubeURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "youtube.com",
		host == "youtu.be",
		strings.HasSuffix(host, ".youtube.com"),
		host == "youtube-nocookie.com",
		strings.HasSuffix(host, ".youtube-nocookie.com"):
		return true
	default:
		return false
	}
}

func resolveMediaURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	path := strings.TrimSuffix(strings.ToLower(u.EscapedPath()), "/")
	if (host == "nts.live" || host == "www.nts.live") &&
		path == "/infinite-mixtapes/slow-focus" {
		return "https://stream-mixtape-geo.ntslive.net/mixtape?client=direct"
	}
	return raw
}
