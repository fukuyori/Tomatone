package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSourceCRUDIncludesTimeRules(t *testing.T) {
	cfg := AmbienceConfig{
		URLs:      []string{"https://radio.example.com/default"},
		FocusURLs: []string{"https://radio.example.com/focus"},
		TimeRules: []TimeRule{{
			Name:  "Night",
			Start: "21:00",
			End:   "06:00",
		}},
	}

	if err := addSourceURL(&cfg, 6, " https://radio.example.com/night-break "); err != nil {
		t.Fatalf("addSourceURL() error = %v", err)
	}
	entries := collectSourceEntries(&cfg)
	if len(entries) != 3 || entries[2].GroupName != "Night / 休憩" {
		t.Fatalf("entries = %#v", entries)
	}

	if err := editSourceURL(&cfg, 2, "https://radio.example.com/focus-new"); err != nil {
		t.Fatalf("editSourceURL() error = %v", err)
	}
	if got := cfg.FocusURLs[0]; got != "https://radio.example.com/focus-new" {
		t.Fatalf("edited URL = %q", got)
	}

	deleted, err := deleteSourceURL(&cfg, 1)
	if err != nil {
		t.Fatalf("deleteSourceURL() error = %v", err)
	}
	if deleted != "https://radio.example.com/default" || len(cfg.URLs) != 0 {
		t.Fatalf("deleted = %q, URLs = %#v", deleted, cfg.URLs)
	}
}

func TestSourceCRUDRejectsInvalidAndDuplicateURLs(t *testing.T) {
	cfg := AmbienceConfig{FocusURLs: []string{"https://radio.example.com/focus"}}
	if err := addSourceURL(&cfg, 2, "not-a-url"); err == nil {
		t.Fatal("invalid URL should be rejected")
	}
	if err := addSourceURL(&cfg, 2, "https://radio.example.com/focus"); err == nil {
		t.Fatal("duplicate URL should be rejected")
	}
}

func TestMoveSourceURLChangesUsageGroup(t *testing.T) {
	cfg := AmbienceConfig{
		FocusURLs: []string{"https://radio.example.com/focus"},
		BreakURLs: []string{"https://radio.example.com/break"},
	}

	if err := moveSourceURL(&cfg, 1, 3); err != nil {
		t.Fatalf("moveSourceURL() error = %v", err)
	}
	if len(cfg.FocusURLs) != 0 {
		t.Fatalf("focus URLs = %#v", cfg.FocusURLs)
	}
	if len(cfg.BreakURLs) != 2 || cfg.BreakURLs[1] != "https://radio.example.com/focus" {
		t.Fatalf("break URLs = %#v", cfg.BreakURLs)
	}
}

func TestMoveSourceURLRejectsSameGroupAndDuplicate(t *testing.T) {
	const sourceURL = "https://radio.example.com/shared"
	cfg := AmbienceConfig{
		FocusURLs: []string{sourceURL},
		BreakURLs: []string{sourceURL},
	}

	if err := moveSourceURL(&cfg, 1, 2); err == nil {
		t.Fatal("moving to the current group should be rejected")
	}
	if err := moveSourceURL(&cfg, 1, 3); err == nil {
		t.Fatal("moving to a group with the same URL should be rejected")
	}
}

func TestRunSourceManagerAddsAndPreviewsSource(t *testing.T) {
	path := t.TempDir() + "/config.json"
	cfg := defaultConfig()
	if err := writeConfig(path, cfg, false); err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader("a\n2\nhttps://radio.example.com/focus\np 1\nq\n")
	var output strings.Builder
	var previewed string
	preview := func(_ string, rawURL string, _ int) error {
		previewed = rawURL
		return nil
	}
	if err := runSourceManager(path, &cfg, input, &output, preview); err != nil {
		t.Fatalf("runSourceManager() error = %v", err)
	}
	if previewed != "https://radio.example.com/focus" {
		t.Fatalf("previewed URL = %q", previewed)
	}
	if !strings.Contains(output.String(), "音源を登録しました") ||
		!strings.Contains(output.String(), "試聴が完了しました") {
		t.Fatalf("manager output = %q", output.String())
	}

	_, loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Ambience.FocusURLs) != 1 || loaded.Ambience.FocusURLs[0] != previewed {
		t.Fatalf("saved focus URLs = %#v", loaded.Ambience.FocusURLs)
	}
}

func TestRunSourceManagerMovesSource(t *testing.T) {
	path := t.TempDir() + "/config.json"
	cfg := defaultConfig()
	cfg.Ambience.FocusURLs = []string{"https://radio.example.com/focus"}
	if err := writeConfig(path, cfg, false); err != nil {
		t.Fatal(err)
	}

	input := strings.NewReader("m 1\n3\nq\n")
	var output strings.Builder
	if err := runSourceManager(path, &cfg, input, &output, func(string, string, int) error { return nil }); err != nil {
		t.Fatalf("runSourceManager() error = %v", err)
	}
	if !strings.Contains(output.String(), "音源の使用場面を変更しました") {
		t.Fatalf("manager output = %q", output.String())
	}

	_, loaded, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Ambience.FocusURLs) != 0 || len(loaded.Ambience.BreakURLs) != 1 {
		t.Fatalf("saved ambience = %#v", loaded.Ambience)
	}
}

func TestBuildSourcePreviewArgs(t *testing.T) {
	args := buildSourcePreviewArgs("https://youtu.be/test", 55)
	for _, want := range []string{"--no-video", "--input-terminal=no", "--volume=55", "--ytdl=yes", "https://youtu.be/test"} {
		if !containsArgument(args, want) {
			t.Fatalf("preview args = %#v, missing %q", args, want)
		}
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--length=") {
			t.Fatalf("preview args must not limit playback duration: %#v", args)
		}
	}
}

func TestSourceManagerCellUsesTerminalWidth(t *testing.T) {
	cell := sourceManagerCell("夜間モード / 集中", 18)
	if got := displayWidth(cell); got != 18 {
		t.Fatalf("cell width = %d, want 18: %q", got, cell)
	}
}

func TestMoveSourceSelectionWraps(t *testing.T) {
	if got := moveSourceSelection(1, -1, 3); got != 3 {
		t.Fatalf("moveSourceSelection(1, -1, 3) = %d, want 3", got)
	}
	if got := moveSourceSelection(3, 1, 3); got != 1 {
		t.Fatalf("moveSourceSelection(3, 1, 3) = %d, want 1", got)
	}
	if got := moveSourceSelection(0, 1, 0); got != 0 {
		t.Fatalf("moveSourceSelection(0, 1, 0) = %d, want 0", got)
	}
}

func TestReadSourceDeleteConfirmation(t *testing.T) {
	keys := []string{sourceKeyDown, "y"}
	index := 0
	confirmed, err := readSourceDeleteConfirmation(func() (string, error) {
		key := keys[index]
		index++
		return key, nil
	})
	if err != nil || !confirmed {
		t.Fatalf("confirmation = (%v, %v), want (true, nil)", confirmed, err)
	}

	cancelled, err := readSourceDeleteConfirmation(func() (string, error) {
		return "n", nil
	})
	if err != nil || cancelled {
		t.Fatalf("cancellation = (%v, %v), want (false, nil)", cancelled, err)
	}
}

func TestWaitForPreviewStop(t *testing.T) {
	keys := []string{"a", sourceKeyDown, sourceKeyEscape, " "}
	index := 0
	if err := waitForPreviewStop(func() (string, error) {
		if index >= len(keys) {
			return "", io.EOF
		}
		key := keys[index]
		index++
		return key, nil
	}); err != nil {
		t.Fatalf("waitForPreviewStop() error = %v", err)
	}
	if index != len(keys) {
		t.Fatalf("preview stopped before Space: read %d of %d keys", index, len(keys))
	}
	if err := waitForPreviewStop(func() (string, error) { return "", io.EOF }); !errors.Is(err, io.EOF) {
		t.Fatalf("waitForPreviewStop() error = %v, want EOF", err)
	}
}
