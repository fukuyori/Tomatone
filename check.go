package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

type URLCheckTarget struct {
	URL       string
	Locations []string
}

type URLListFlag []string

func (f *URLListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *URLListFlag) Set(value string) error {
	if err := validateAudioURL(value); err != nil {
		return err
	}
	*f = append(*f, value)
	return nil
}

func explicitURLCheckTargets(urls []string) []URLCheckTarget {
	targets := make([]URLCheckTarget, 0, len(urls))
	seen := make(map[string]bool)
	for _, rawURL := range urls {
		if seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		targets = append(targets, URLCheckTarget{
			URL:       rawURL,
			Locations: []string{"--check-url"},
		})
	}
	return targets
}

func collectURLCheckTargets(cfg AmbienceConfig) []URLCheckTarget {
	var targets []URLCheckTarget
	indexes := make(map[string]int)
	add := func(url, location string) {
		if index, ok := indexes[url]; ok {
			targets[index].Locations = append(targets[index].Locations, location)
			return
		}
		indexes[url] = len(targets)
		targets = append(targets, URLCheckTarget{
			URL:       url,
			Locations: []string{location},
		})
	}
	addList := func(urls []string, location string) {
		for i, url := range urls {
			add(url, fmt.Sprintf("%s[%d]", location, i))
		}
	}

	addList(cfg.URLs, "ambience.urls")
	addList(cfg.FocusURLs, "ambience.focus_urls")
	addList(cfg.BreakURLs, "ambience.break_urls")
	for i, rule := range cfg.TimeRules {
		prefix := fmt.Sprintf("ambience.time_rules[%d]", i)
		addList(rule.URLs, prefix+".urls")
		addList(rule.FocusURLs, prefix+".focus_urls")
		addList(rule.BreakURLs, prefix+".break_urls")
	}
	return targets
}

func checkConfiguredURLs(cfg AmbienceConfig, timeout time.Duration) error {
	return checkURLTargets(cfg.PlayerCommand, collectURLCheckTargets(cfg), timeout)
}

func checkURLTargets(playerCommand string, targets []URLCheckTarget, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("--check-timeout は0より大きくしてください")
	}
	if len(targets) == 0 {
		fmt.Println("URL: 登録なし")
		return nil
	}
	if _, err := exec.LookPath(playerCommand); err != nil {
		return fmt.Errorf("%s が見つかりません: %w", playerCommand, err)
	}

	fmt.Printf("URL: %d件を確認します（1件あたり最大 %s）\n", len(targets), timeout)
	failures := 0
	for i, target := range targets {
		started := time.Now()
		err := probeMediaURL(playerCommand, target.URL, timeout)
		elapsed := time.Since(started).Round(100 * time.Millisecond)
		kind := "STREAM"
		if isYouTubeURL(target.URL) {
			kind = "YOUTUBE"
		}
		if err != nil {
			failures++
			fmt.Printf("  [%d/%d] NG  %-7s %s (%s)\n",
				i+1, len(targets), kind, target.URL, elapsed)
			fmt.Printf("          %s\n", err)
		} else {
			fmt.Printf("  [%d/%d] OK  %-7s %s (%s)\n",
				i+1, len(targets), kind, target.URL, elapsed)
		}
		if len(target.Locations) > 1 {
			fmt.Printf("          使用箇所: %s\n", strings.Join(target.Locations, ", "))
		}
		if resolved := resolveMediaURL(target.URL); resolved != target.URL {
			fmt.Printf("          解決先: %s\n", resolved)
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d/%d件のURL検査に失敗しました", failures, len(targets))
	}
	fmt.Printf("URL: 全%d件が再生可能です\n", len(targets))
	return nil
}

func probeMediaURL(playerCommand, rawURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{
		"--no-config",
		"--no-video",
		"--force-window=no",
		"--ao=null",
		"--volume=0",
		"--length=1",
		"--msg-level=all=error",
	}
	playbackURL := resolveMediaURL(rawURL)
	if isYouTubeURL(playbackURL) {
		args = append(args, "--ytdl=yes")
	} else {
		args = append(args, "--ytdl=no")
	}
	args = append(args, playbackURL)

	output, err := exec.CommandContext(ctx, playerCommand, args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%sでタイムアウトしました", timeout)
	}
	if err != nil {
		detail := lastOutputLine(string(output), 160)
		if detail == "" {
			return fmt.Errorf("mpv: %w", err)
		}
		return fmt.Errorf("mpv: %s", detail)
	}
	return nil
}

func lastOutputLine(output string, maxCells int) string {
	lines := strings.FieldsFunc(output, func(r rune) bool { return r == '\r' || r == '\n' })
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > maxCells {
			runes := []rune(line)
			return string(runes[:maxCells-1]) + "…"
		}
		return line
	}
	return ""
}
