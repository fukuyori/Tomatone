package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PlayerState string

const (
	PlayerDisabled PlayerState = "disabled"
	PlayerStarting PlayerState = "starting"
	PlayerPlaying  PlayerState = "playing"
	PlayerPaused   PlayerState = "paused"
	PlayerError    PlayerState = "error"
	PlayerStopped  PlayerState = "stopped"
)

type PlayerSnapshot struct {
	State    PlayerState
	Title    string
	URL      string
	Index    int
	Total    int
	Volume   int
	Position time.Duration
	Duration time.Duration
	Err      string
}

type AmbientPlayer struct {
	cfg           AmbienceConfig
	mu            sync.RWMutex
	inputMu       sync.Mutex
	snapshot      PlayerSnapshot
	next          chan struct{}
	generation    int
	input         io.ReadWriteCloser
	inputReader   *bufio.Reader
	volumePending bool
	requestID     int
	currentPhase  Phase
	activeURLs    []string
	positions     map[string]time.Duration
}

func NewAmbientPlayer(cfg AmbienceConfig) *AmbientPlayer {
	state := PlayerStopped
	if !cfg.HasAnyURL() {
		state = PlayerDisabled
	}
	initialURLs := ResolveURLs(cfg, PhaseFocus, time.Now())
	return &AmbientPlayer{
		cfg:          cfg,
		currentPhase: PhaseFocus,
		activeURLs:   initialURLs,
		positions:    make(map[string]time.Duration),
		snapshot: PlayerSnapshot{
			State:  state,
			Total:  len(initialURLs),
			Volume: cfg.Volume,
		},
		next: make(chan struct{}, 1),
	}
}

func (p *AmbientPlayer) savePosition(url string, pos, duration time.Duration) {
	if url == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.positions == nil {
		p.positions = make(map[string]time.Duration)
	}
	if duration <= 0 {
		delete(p.positions, url)
	} else if duration-pos <= 3*time.Second {
		delete(p.positions, url)
	} else if pos >= 3*time.Second {
		p.positions[url] = pos
	}
}

func (p *AmbientPlayer) getSavedPosition(url string) time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.positions == nil {
		return 0
	}
	return p.positions[url]
}

func (p *AmbientPlayer) clearSavedPosition(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.positions != nil {
		delete(p.positions, url)
	}
}

func (p *AmbientPlayer) SetPhase(phase Phase) {
	p.mu.Lock()
	oldPhase := p.currentPhase
	p.currentPhase = phase
	p.mu.Unlock()

	if oldPhase != phase {
		p.checkAndSwitchURLs()
	}
}

func (p *AmbientPlayer) checkAndSwitchURLs() {
	p.mu.RLock()
	phase := p.currentPhase
	currentActive := p.activeURLs
	p.mu.RUnlock()

	newURLs := ResolveURLs(p.cfg, phase, time.Now())
	if !equalStringSlices(currentActive, newURLs) {
		p.Next()
	}
}

func (p *AmbientPlayer) Run(ctx context.Context) {
	if !p.cfg.HasAnyURL() {
		return
	}
	if _, err := exec.LookPath(p.cfg.PlayerCommand); err != nil {
		p.setError(fmt.Sprintf("%s が見つかりません。mpv をインストールするか player_command を変更してください", p.cfg.PlayerCommand))
		return
	}

	orderPos := 0
	var lastURLList []string
	lastURL := ""

	for {
		p.mu.RLock()
		phase := p.currentPhase
		p.mu.RUnlock()

		urls := ResolveURLs(p.cfg, phase, time.Now())
		if len(urls) == 0 {
			p.setError("適合する再生 URL がありません")
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		p.mu.Lock()
		p.activeURLs = urls
		p.mu.Unlock()

		if !equalStringSlices(lastURLList, urls) {
			orderPos = 0
			lastURLList = urls
		}

		if orderPos >= len(urls) {
			orderPos = 0
		}

		index := choosePlaybackIndex(urls, p.cfg.Shuffle, orderPos, lastURL)
		naturalEnd, keepGoing := p.playOne(ctx, urls, index)
		if !keepGoing {
			return
		}
		lastURL = urls[index]
		if naturalEnd {
			p.clearSavedPosition(urls[index])
		}
		if !p.cfg.Shuffle {
			orderPos = (orderPos + 1) % len(urls)
		}
		if naturalEnd {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1500 * time.Millisecond):
			}
		}
	}
}

func choosePlaybackIndex(urls []string, shuffle bool, orderPos int, lastURL string) int {
	if len(urls) == 0 {
		return 0
	}
	if !shuffle {
		if orderPos < 0 {
			return 0
		}
		return orderPos % len(urls)
	}

	candidates := make([]int, 0, len(urls))
	for index, url := range urls {
		if len(urls) == 1 || url != lastURL {
			candidates = append(candidates, index)
		}
	}
	if len(candidates) == 0 {
		return rand.IntN(len(urls))
	}
	return candidates[rand.IntN(len(candidates))]
}

func (p *AmbientPlayer) playOne(ctx context.Context, urls []string, index int) (naturalEnd, keepGoing bool) {
	trackCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	url := urls[index]
	playbackURL := resolveMediaURL(url)
	startPos := p.getSavedPosition(url)

	p.mu.Lock()
	p.generation++
	generation := p.generation
	volume := p.cfg.Volume
	p.snapshot = PlayerSnapshot{
		State:  PlayerStarting,
		URL:    url,
		Index:  index + 1,
		Total:  len(urls),
		Volume: volume,
	}
	p.volumePending = false
	p.mu.Unlock()

	args := buildPlaybackArgs(playbackURL, volume, startPos)
	ipcPath := newMPVIPCPath()
	defer cleanupMPVIPC(ipcPath)
	args = append(args, "--input-ipc-server="+ipcPath, playbackURL)
	cmd := exec.CommandContext(trackCtx, p.cfg.PlayerCommand, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.setErrorFor(generation, err.Error())
		return true, true
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.setErrorFor(generation, err.Error())
		return true, true
	}
	if err := cmd.Start(); err != nil {
		p.setErrorFor(generation, err.Error())
		return true, true
	}
	ipc, err := connectMPVIPC(ipcPath)
	if err != nil {
		cancel()
		_ = cmd.Wait()
		p.setErrorFor(generation, err.Error())
		return true, true
	}
	p.setInputFor(generation, ipc)
	defer p.clearInputFor(generation)

	p.setStateFor(generation, PlayerPlaying)
	var scanWG sync.WaitGroup
	scanWG.Add(2)
	go func() {
		defer scanWG.Done()
		p.scanOutput(stdout, generation)
	}()
	go func() {
		defer scanWG.Done()
		p.scanOutput(stderr, generation)
	}()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err := <-waitCh:
		scanWG.Wait()
		if ctx.Err() != nil {
			p.setStateFor(generation, PlayerStopped)
			return false, false
		}
		if err != nil {
			p.setErrorFor(generation, err.Error())
		} else {
			p.setStateFor(generation, PlayerStopped)
		}
		return true, true
	case <-p.next:
		p.sendQuitCommand()
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitCh
		scanWG.Wait()
		if ctx.Err() != nil {
			p.setStateFor(generation, PlayerStopped)
			return false, false
		}
		return false, true
	case <-ctx.Done():
		p.sendQuitCommand()
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitCh
		scanWG.Wait()
		p.setStateFor(generation, PlayerStopped)
		return false, false
	}
}

func buildPlaybackArgs(url string, volume int, startPos time.Duration) []string {
	args := []string{
		"--no-video",
		"--force-window=no",
		"--idle=no",
		"--volume=" + strconv.Itoa(volume),
		"--term-playing-msg=TOMATONE_META\t${media-title}",
		"--term-status-msg=TOMATONE_STATUS\t${media-title}\t${time-pos}\t${duration}\t${pause}\t${volume}\t${metadata/by-key/icy-title}",
	}
	if isYouTubeURL(url) {
		args = append(args, "--ytdl=yes")
	} else {
		args = append(args, "--ytdl=no")
	}
	if startPos >= 3*time.Second {
		args = append(args, fmt.Sprintf("--start=%d", int(startPos.Seconds())))
	}
	return args
}

func (p *AmbientPlayer) sendQuitCommand() {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	p.mu.RLock()
	input := p.input
	p.mu.RUnlock()
	if input != nil {
		_, _ = io.WriteString(input, `{"command":["quit"]}`+"\n")
	}
}

func (p *AmbientPlayer) Next() {
	p.mu.RLock()
	hasURLs := len(p.activeURLs) > 0 || len(p.cfg.URLs) > 0
	p.mu.RUnlock()
	if !hasURLs {
		return
	}
	select {
	case p.next <- struct{}{}:
	default:
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (p *AmbientPlayer) AdjustVolume(increase bool) (int, bool) {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()

	p.mu.RLock()
	input := p.input
	reader := p.inputReader
	generation := p.generation
	p.mu.RUnlock()
	if input == nil || reader == nil {
		return 0, false
	}

	delta := -5
	if increase {
		delta = 5
	}
	p.mu.Lock()
	p.requestID++
	requestID := p.requestID
	p.mu.Unlock()
	command := fmt.Sprintf(
		`{"command":["add","volume",%d],"request_id":%d}`+"\n",
		delta, requestID)
	if _, err := io.WriteString(input, command); err != nil {
		return 0, false
	}
	var response struct {
		Error     string `json:"error"`
		RequestID int    `json:"request_id"`
	}
	matched := false
	for range 32 {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return 0, false
		}
		if json.Unmarshal(line, &response) != nil || response.RequestID != requestID {
			continue
		}
		matched = true
		break
	}
	if !matched || response.Error != "success" {
		return 0, false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generation || p.input == nil {
		return 0, false
	}
	volume := p.snapshot.Volume + delta
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	p.snapshot.Volume = volume
	p.cfg.Volume = volume
	p.volumePending = true
	return volume, true
}

func (p *AmbientPlayer) Snapshot() PlayerSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
}

func (p *AmbientPlayer) scanOutput(r io.Reader, generation int) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitCRLF)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		p.parseLine(strings.TrimSpace(scanner.Text()), generation)
	}
}

func (p *AmbientPlayer) parseLine(line string, generation int) {
	if strings.HasPrefix(line, "TOMATONE_META\t") {
		title := strings.TrimSpace(strings.TrimPrefix(line, "TOMATONE_META\t"))
		p.mu.Lock()
		defer p.mu.Unlock()
		if generation == p.generation && title != "" {
			p.snapshot.Title = title
			p.snapshot.State = PlayerPlaying
			p.snapshot.Err = ""
		}
		return
	}
	if !strings.HasPrefix(line, "TOMATONE_STATUS\t") {
		return
	}
	fields := strings.Split(line, "\t")
	if len(fields) < 5 {
		return
	}
	position := secondsField(fields[2])
	duration := secondsField(fields[3])
	paused := strings.EqualFold(strings.TrimSpace(fields[4]), "yes") ||
		strings.EqualFold(strings.TrimSpace(fields[4]), "true")

	p.mu.Lock()
	defer p.mu.Unlock()
	if generation != p.generation {
		return
	}
	title := strings.TrimSpace(fields[1])
	if len(fields) >= 7 {
		if streamTitle := validMetadataField(fields[6]); streamTitle != "" {
			title = streamTitle
		}
	}
	if title != "" {
		p.snapshot.Title = title
	}
	p.snapshot.Position = position
	p.snapshot.Duration = duration
	p.snapshot.Err = ""
	if position > 0 && p.snapshot.URL != "" {
		if p.positions == nil {
			p.positions = make(map[string]time.Duration)
		}
		if duration > 0 && duration-position <= 3*time.Second {
			delete(p.positions, p.snapshot.URL)
		} else if position >= 3*time.Second {
			p.positions[p.snapshot.URL] = position
		}
	}
	if len(fields) >= 6 {
		if volume, ok := numberField(fields[5]); ok {
			rounded := int(volume + 0.5)
			if rounded < 0 {
				rounded = 0
			}
			if rounded > 100 {
				rounded = 100
			}
			if !p.volumePending || rounded == p.cfg.Volume {
				p.snapshot.Volume = rounded
				p.cfg.Volume = rounded
				p.volumePending = false
			}
		}
	}
	if paused {
		p.snapshot.State = PlayerPaused
	} else {
		p.snapshot.State = PlayerPlaying
	}
}

func validMetadataField(value string) string {
	value = strings.TrimSpace(value)
	switch strings.ToLower(value) {
	case "", "(error)", "n/a", "nil":
		return ""
	default:
		return value
	}
}

func (p *AmbientPlayer) setError(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot.State = PlayerError
	p.snapshot.Err = message
}

func (p *AmbientPlayer) setErrorFor(generation int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation == p.generation {
		p.snapshot.State = PlayerError
		p.snapshot.Err = message
	}
}

func (p *AmbientPlayer) setStateFor(generation int, state PlayerState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation == p.generation {
		p.snapshot.State = state
	}
}

func (p *AmbientPlayer) setInputFor(generation int, input io.ReadWriteCloser) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if generation == p.generation {
		p.input = input
		p.inputReader = bufio.NewReader(input)
	}
}

func (p *AmbientPlayer) clearInputFor(generation int) {
	p.inputMu.Lock()
	defer p.inputMu.Unlock()
	p.mu.Lock()
	var input io.ReadWriteCloser
	if generation == p.generation {
		input = p.input
		p.input = nil
		p.inputReader = nil
	}
	p.mu.Unlock()
	if input != nil {
		_ = input.Close()
	}
}

func secondsField(value string) time.Duration {
	f, ok := numberField(value)
	if !ok || f < 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

func numberField(value string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return f, err == nil
}

func splitCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		advance = i + 1
		for advance < len(data) && (data[advance] == '\r' || data[advance] == '\n') {
			advance++
		}
		return advance, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
