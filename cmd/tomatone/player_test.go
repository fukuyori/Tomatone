package main

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type recordingWriteCloser struct {
	strings.Builder
	responses int
	emitEvent bool
	eventSent bool
}

func (w *recordingWriteCloser) Close() error { return nil }
func (w *recordingWriteCloser) Read(buffer []byte) (int, error) {
	if w.emitEvent && !w.eventSent {
		w.eventSent = true
		return copy(buffer, "{\"event\":\"file-loaded\"}\n"), nil
	}
	w.responses++
	response := `{"error":"success","request_id":` +
		fmt.Sprintf("%d", w.responses) + "}\n"
	return copy(buffer, response), nil
}

func TestAmbientPlayerReportsMissingCommand(t *testing.T) {
	player := NewAmbientPlayer(YouTubeConfig{
		URLs:          []string{"https://youtu.be/test"},
		PlayerCommand: "tomatone-player-that-does-not-exist",
	})

	player.Run(context.Background())
	got := player.Snapshot()
	if got.State != PlayerError {
		t.Fatalf("state = %q, want %q", got.State, PlayerError)
	}
	if !strings.Contains(got.Err, "見つかりません") {
		t.Fatalf("error = %q", got.Err)
	}
}

func TestSplitCRLF(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("one\rtwo\r\nthree\n"))
	scanner.Split(splitCRLF)
	var got []string
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	want := []string{"one", "two", "three"}
	if len(got) != len(want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAdjustVolumeControlsMPVAndKeepsVolume(t *testing.T) {
	writer := &recordingWriteCloser{emitEvent: true}
	player := NewAmbientPlayer(YouTubeConfig{
		URLs:   []string{"https://youtu.be/test"},
		Volume: 45,
	})
	player.generation = 1
	player.setInputFor(1, writer)

	if volume, ok := player.AdjustVolume(true); !ok || volume != 50 {
		t.Fatalf("increase = (%d, %v), want (50, true)", volume, ok)
	}
	if volume, ok := player.AdjustVolume(false); !ok || volume != 45 {
		t.Fatalf("decrease = (%d, %v), want (45, true)", volume, ok)
	}
	wantCommands := "{\"command\":[\"add\",\"volume\",5],\"request_id\":1}\n" +
		"{\"command\":[\"add\",\"volume\",-5],\"request_id\":2}\n"
	if got := writer.String(); got != wantCommands {
		t.Fatalf("mpv input = %q, want %q", got, wantCommands)
	}
	if player.cfg.Volume != 45 {
		t.Fatalf("next-track volume = %d, want 45", player.cfg.Volume)
	}
}

func TestPendingVolumeDoesNotRevertOnOldStatus(t *testing.T) {
	writer := &recordingWriteCloser{}
	player := NewAmbientPlayer(YouTubeConfig{
		URLs:   []string{"https://youtu.be/test"},
		Volume: 45,
	})
	player.generation = 1
	player.setInputFor(1, writer)

	if _, ok := player.AdjustVolume(true); !ok {
		t.Fatal("AdjustVolume() failed")
	}
	player.parseLine("TOMATONE_STATUS\tRain\t1\t100\tno\t45", 1)
	if got := player.Snapshot().Volume; got != 50 {
		t.Fatalf("volume reverted to %d, want 50", got)
	}

	player.parseLine("TOMATONE_STATUS\tRain\t2\t100\tno\t50", 1)
	if got := player.Snapshot().Volume; got != 50 {
		t.Fatalf("confirmed volume = %d, want 50", got)
	}
	if player.volumePending {
		t.Fatal("volume should no longer be pending")
	}
}

func TestImmediateInputAcceptsVolumeKeys(t *testing.T) {
	commands := make(chan string, 5)
	readImmediateCommands(context.Background(), strings.NewReader(" +x-=_"), commands)
	close(commands)

	var got []string
	for command := range commands {
		got = append(got, command)
	}
	want := []string{" ", "+", "-", "=", "_"}
	if strings.Join(got, "") != strings.Join(want, "") {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestPositionTrackingAndResume(t *testing.T) {
	player := NewAmbientPlayer(YouTubeConfig{
		URLs: []string{"https://youtu.be/test1"},
	})
	player.snapshot.URL = "https://youtu.be/test1"

	// Simulate status line with 45s playback
	player.parseLine("TOMATONE_STATUS\tTest Title\t45\t300\tno\t50", 0)

	saved := player.getSavedPosition("https://youtu.be/test1")
	if saved != 45*time.Second {
		t.Fatalf("saved position = %v, want 45s", saved)
	}

	// Near the end of track
	player.parseLine("TOMATONE_STATUS\tTest Title\t299\t300\tno\t50", 0)
	saved = player.getSavedPosition("https://youtu.be/test1")
	if saved != 0 {
		t.Fatalf("saved position after track end = %v, want 0", saved)
	}
}

func TestLiveStreamPositionIsNotSaved(t *testing.T) {
	const radioURL = "https://radio.example.com/live.mp3"
	player := NewAmbientPlayer(AmbienceConfig{URLs: []string{radioURL}})
	player.savePosition(radioURL, 5*time.Minute, 0)
	if got := player.getSavedPosition(radioURL); got != 0 {
		t.Fatalf("live position = %v, want 0", got)
	}
}

func TestPlaybackArgsSelectResolverBySource(t *testing.T) {
	youtubeArgs := buildPlaybackArgs("https://youtu.be/test", 45, 0)
	if !containsArgument(youtubeArgs, "--ytdl=yes") {
		t.Fatalf("YouTube args = %#v, want --ytdl=yes", youtubeArgs)
	}

	radioArgs := buildPlaybackArgs("https://radio.example.com/live.mp3", 45, 0)
	if !containsArgument(radioArgs, "--ytdl=no") {
		t.Fatalf("radio args = %#v, want --ytdl=no", radioArgs)
	}
}

func TestChoosePlaybackIndexUsesConfiguredOrderWhenShuffleDisabled(t *testing.T) {
	urls := []string{"one", "two", "three"}
	for orderPos, want := range []int{0, 1, 2, 0} {
		if got := choosePlaybackIndex(urls, false, orderPos, ""); got != want {
			t.Fatalf("orderPos %d: index = %d, want %d", orderPos, got, want)
		}
	}
}

func TestChoosePlaybackIndexAvoidsImmediateRepeatWhenShuffling(t *testing.T) {
	urls := []string{"one", "two", "three"}
	lastURL := "one"
	for range 100 {
		index := choosePlaybackIndex(urls, true, 0, lastURL)
		if index < 0 || index >= len(urls) {
			t.Fatalf("index = %d, want 0..%d", index, len(urls)-1)
		}
		if urls[index] == lastURL {
			t.Fatalf("selected %q twice in a row", lastURL)
		}
		lastURL = urls[index]
	}
}

func TestChoosePlaybackIndexHandlesDuplicateOnlyList(t *testing.T) {
	urls := []string{"same", "same"}
	index := choosePlaybackIndex(urls, true, 0, "same")
	if index < 0 || index >= len(urls) {
		t.Fatalf("index = %d, want 0..%d", index, len(urls)-1)
	}
}

func containsArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
