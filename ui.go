package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiFocus  = "\x1b[38;5;203m" // Warm Coral Red
	ansiBreak  = "\x1b[38;5;114m" // Mint Green
	ansiLong   = "\x1b[38;5;75m"  // Soft Sky Blue
	ansiAudio  = "\x1b[38;5;179m" // Warm Sand Gold
	ansiSubtle = "\x1b[38;5;242m" // Dark Gray for Rules
	ansiError  = "\x1b[38;5;203m"
)

type UI struct {
	out        io.Writer
	configPath string
	message    string
	immediate  bool
}

func NewUI(out io.Writer, configPath string, immediate bool) *UI {
	return &UI{out: out, configPath: configPath, immediate: immediate}
}

func (u *UI) Enter() {
	fmt.Fprint(u.out, "\x1b[?1049h\x1b[?25l\x1b[2J")
}

func (u *UI) Close() {
	fmt.Fprint(u.out, ansiReset+"\x1b[?25h\x1b[?1049l")
}

func (u *UI) SetMessage(message string) {
	u.message = message
}

func (u *UI) Render(timer *Pomodoro, player PlayerSnapshot) {
	const width = 54
	accent := phaseAccent(timer.Phase())
	var frame strings.Builder
	frame.WriteString("\x1b[2J\x1b[H")

	// 1. Header Row
	headerLeft := "  🍅 " + ansiBold + "TOMATONE" + ansiReset
	sessionStr := fmt.Sprintf("%d/%d", timer.SessionNumber(), timer.SessionsBeforeLongBreak())
	headerRight := accent + ansiBold + phaseLabel(timer.Phase()) + ansiReset + ansiSubtle + "  [" + ansiReset + sessionStr + ansiSubtle + "]" + ansiReset + "  "
	frame.WriteString(alignRow(headerLeft, headerRight, width) + "\n")

	// Divider
	frame.WriteString("  " + ansiSubtle + strings.Repeat("─", width-4) + ansiReset + "\n")

	// 2. Main Timer Clock
	clockStr := accent + ansiBold + formatClock(timer.Remaining()) + ansiReset
	frame.WriteString(center(clockStr, width) + "\n")

	// 3. Status Summary
	stateLabel := "● RUNNING"
	if !timer.Running() {
		stateLabel = "Ⅱ PAUSED"
	}
	statusStr := ansiBold + stateLabel + ansiReset + ansiSubtle + "   ·   " + ansiReset + fmt.Sprintf("SESSION %d / %d", timer.SessionNumber(), timer.SessionsBeforeLongBreak())
	frame.WriteString(center(statusStr, width) + "\n")

	// 4. Timer Progress Bar
	timerBar := buildProgressBar(timer.Progress(), 38, accent)
	frame.WriteString(center(timerBar, width) + "\n")

	// Divider
	frame.WriteString("  " + ansiSubtle + strings.Repeat("─", width-4) + ansiReset + "\n")

	// 5. Track Title & Ambience Status
	title := player.Title
	if title == "" && player.Total > 0 {
		title = "ストリームに接続しています…"
	}
	if title == "" {
		title = "アンビエンス URL が未設定です"
	}
	titleLeft := "  🎵 " + ansiBold + truncate(title, 34) + ansiReset
	midRight := playerStateLabel(player.State) + "  "
	frame.WriteString(alignRow(titleLeft, midRight, width) + "\n")

	// 6. Player Details / Stats
	if player.Total > 0 {
		left := "     " + playbackTime(player)
		right := fmt.Sprintf("VOL %d%%   ·   %d/%d", player.Volume, player.Index, player.Total) + "  "
		frame.WriteString(alignRow(left, right, width) + "\n")

		// Audio Progress Bar
		if player.Duration > 0 {
			audioBar := buildProgressBar(ratio(player.Position, player.Duration), 44, ansiAudio)
			frame.WriteString("     " + audioBar + "\n")
		}
	} else {
		frame.WriteString("     " + ansiDim + "config.json を編集してください" + ansiReset + "\n")
	}

	if player.Err != "" {
		errStr := "     " + ansiError + truncate("再生エラー: "+player.Err, 44) + ansiReset
		frame.WriteString(errStr + "\n")
	}

	if u.message != "" {
		msgStr := "     " + accent + truncate(u.message, 44) + ansiReset
		frame.WriteString(msgStr + "\n")
	}

	// Bottom Divider
	frame.WriteString("  " + ansiSubtle + strings.Repeat("─", width-4) + ansiReset + "\n")

	// 7. Control Help Keybindings
	frame.WriteString(renderKeybindings(u.immediate) + "\n")

	frame.WriteString("\x1b[J")
	fmt.Fprint(u.out, frame.String())
}

func buildProgressBar(progress float64, barWidth int, accent string) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress*float64(barWidth) + 0.5)
	if filled > barWidth {
		filled = barWidth
	}
	return accent + strings.Repeat("━", filled) + ansiReset + ansiSubtle + strings.Repeat("─", barWidth-filled) + ansiReset
}

func renderKeybindings(immediate bool) string {
	k := func(key, label string) string {
		return ansiSubtle + "[" + ansiReset + ansiBold + key + ansiReset + ansiSubtle + "]" + ansiReset + label
	}
	line := "  " + k("p/Space", " pause") + "  " + k("c", " chime") + "  " + k("s", " skip") + "  " + k("r", " reset") + "  " + k("n", " next") + "  " + k("-/+", " vol") + "  " + k("q", " quit")
	if !immediate {
		line += ansiDim + "  (Enter)" + ansiReset
	}
	return line
}

func timerSummary(timer *Pomodoro) string {
	state := "RUNNING"
	if !timer.Running() {
		state = "PAUSED"
	}
	return fmt.Sprintf("%s  ·  %d / %d",
		state,
		timer.SessionNumber(), timer.SessionsBeforeLongBreak())
}

func phaseAccent(phase Phase) string {
	switch phase {
	case PhaseShortBreak:
		return ansiBreak
	case PhaseLongBreak:
		return ansiLong
	default:
		return ansiFocus
	}
}

func renderProgressLine(w io.Writer, progress float64, barWidth, lineWidth int, accent string) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress*float64(barWidth) + 0.5)
	padding := (lineWidth - barWidth) / 2
	fmt.Fprintf(w, "  %s%s%s%s%s%s\n",
		strings.Repeat(" ", padding),
		accent, strings.Repeat("━", filled),
		ansiReset+ansiSubtle, strings.Repeat("─", barWidth-filled),
		ansiReset)
}

func alignRow(left, right string, width int) string {
	gap := width - visibleWidth(left) - visibleWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func center(value string, width int) string {
	length := visibleWidth(value)
	if length >= width {
		return value
	}
	return strings.Repeat(" ", (width-length)/2) + value
}

func playbackTime(player PlayerSnapshot) string {
	if player.Duration > 0 {
		return formatMediaTime(player.Position) + " / " + formatMediaTime(player.Duration)
	}
	if player.Position > 0 {
		return formatMediaTime(player.Position) + " / LIVE"
	}
	if player.State == PlayerPlaying || player.State == PlayerPaused {
		return "LIVE"
	}
	return "--:--"
}

func phaseLabel(phase Phase) string {
	switch phase {
	case PhaseShortBreak:
		return "SHORT BREAK"
	case PhaseLongBreak:
		return "LONG BREAK"
	default:
		return "FOCUS"
	}
}

func playerStateLabel(state PlayerState) string {
	switch state {
	case PlayerStarting:
		return ansiAudio + "◌ 接続中" + ansiReset
	case PlayerPlaying:
		return ansiAudio + "● 再生中" + ansiReset
	case PlayerPaused:
		return ansiDim + "Ⅱ 一時停止" + ansiReset
	case PlayerError:
		return ansiError + "× 再生エラー" + ansiReset
	case PlayerDisabled:
		return ansiDim + "○ URL未登録" + ansiReset
	default:
		return ansiDim + "○ 停止" + ansiReset
	}
}

func ratio(a, b time.Duration) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	hours := int(d / time.Hour)
	minutes := int(d/time.Minute) % 60
	seconds := int(d/time.Second) % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatMediaTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second) / time.Second)
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func visibleWidth(s string) int {
	var clean strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		clean.WriteRune(r)
	}
	return displayWidth(clean.String())
}

func truncate(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	var result strings.Builder
	used := 0
	for _, r := range s {
		width := runeDisplayWidth(r)
		if used+width > max-1 {
			break
		}
		result.WriteRune(r)
		used += width
	}
	return result.String() + "…"
}

func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		width += runeDisplayWidth(r)
	}
	return width
}

func runeDisplayWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	switch {
	case r >= 0x1100 && r <= 0x115f,
		r >= 0x2329 && r <= 0x232a,
		r >= 0x2e80 && r <= 0x303e,
		r >= 0x3040 && r <= 0xa4cf,
		r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff,
		r >= 0xfe10 && r <= 0xfe19,
		r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff01 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1faff,
		r >= 0x20000 && r <= 0x3fffd:
		return 2
	default:
		return 1
	}
}
