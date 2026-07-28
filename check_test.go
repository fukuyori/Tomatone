package main

import (
	"reflect"
	"testing"
)

func TestCollectURLCheckTargetsDeduplicatesURLs(t *testing.T) {
	cfg := AmbienceConfig{
		URLs:      []string{"https://radio.example.com/live.mp3"},
		FocusURLs: []string{"https://radio.example.com/live.mp3", "https://youtu.be/focus"},
		TimeRules: []TimeRule{
			{
				Start: "22:00",
				End:   "06:00",
				URLs:  []string{"https://radio.example.com/night.pls"},
			},
		},
	}

	got := collectURLCheckTargets(cfg)
	if len(got) != 3 {
		t.Fatalf("targets = %#v, want 3 unique URLs", got)
	}
	wantLocations := []string{"ambience.urls[0]", "ambience.focus_urls[0]"}
	if !reflect.DeepEqual(got[0].Locations, wantLocations) {
		t.Fatalf("locations = %#v, want %#v", got[0].Locations, wantLocations)
	}
}

func TestLastOutputLine(t *testing.T) {
	got := lastOutputLine("first\n\nlast error\n", 160)
	if got != "last error" {
		t.Fatalf("lastOutputLine() = %q", got)
	}
}
