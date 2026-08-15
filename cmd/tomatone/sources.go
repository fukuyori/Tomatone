package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type sourceGroup struct {
	Name string
	URLs *[]string
}

type sourceEntry struct {
	ID        int
	GroupID   int
	GroupName string
	ListIndex int
	URL       string
}

type sourcePreviewFunc func(playerCommand, rawURL string, volume int) error

type sourceManagerConsole struct {
	input   *os.File
	restore func()
}

func (console *sourceManagerConsole) close() {
	if console.restore != nil {
		console.restore()
	}
}

func (console *sourceManagerConsole) readLines(action func(*bufio.Reader) error) error {
	console.restore()
	actionErr := action(bufio.NewReader(console.input))
	restore, immediate, modeErr := enableImmediateInput(console.input)
	if modeErr != nil {
		return modeErr
	}
	if !immediate {
		return errors.New("コンソールをCUI入力モードに戻せません")
	}
	console.restore = restore
	return actionErr
}

func runSourceManagerCommand(path string, args []string) error {
	if len(args) > 0 {
		return errors.New("sources に追加の引数は指定できません")
	}
	created, err := ensureConfig(path)
	if err != nil {
		return err
	}
	cfg, _, err := loadConfig(path)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr, "設定ファイルを作成しました: %s\n", path)
	}

	fmt.Fprint(os.Stdout, "\x1b[?1049h")
	defer fmt.Fprint(os.Stdout, ansiReset+"\x1b[?1049l")
	restoreInput, immediate, err := enableImmediateInput(os.Stdin)
	if err != nil {
		return err
	}
	if immediate {
		console := &sourceManagerConsole{input: os.Stdin, restore: restoreInput}
		defer console.close()
		return runSourceManagerCUI(path, &cfg, os.Stdout, previewSource, console)
	}
	return runSourceManager(path, &cfg, os.Stdin, os.Stdout, previewSource)
}

func runSourceManager(path string, cfg *Config, input io.Reader, output io.Writer, preview sourcePreviewFunc) error {
	reader := bufio.NewReader(input)
	message := ""
	for {
		ambience, err := sourceConfigPointer(cfg)
		if err != nil {
			return err
		}
		renderSourceManager(output, path, ambience, message, 0, false)
		message = ""

		line, err := readSourceManagerLine(reader, output, "操作 > ")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		parts := strings.Fields(strings.ToLower(line))
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "a", "add":
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			renderSourceGroups(output, candidateAmbience, "追加先")
			groupID, err := readSourceManagerID(reader, output, nil, "追加先の番号 > ")
			if err != nil {
				message = err.Error()
				continue
			}
			url, err := readSourceManagerLine(reader, output, "URL > ")
			if err != nil {
				return err
			}
			if err := addSourceURL(candidateAmbience, groupID, url); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源を登録しました"

		case "e", "edit":
			entryID, err := readSourceManagerID(reader, output, parts[1:], "修正するID > ")
			if err != nil {
				message = err.Error()
				continue
			}
			url, err := readSourceManagerLine(reader, output, "新しいURL > ")
			if err != nil {
				return err
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if err := editSourceURL(candidateAmbience, entryID, url); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源URLを修正しました"

		case "m", "move":
			entryID, err := readSourceManagerID(reader, output, parts[1:], "使用場面を変更するID > ")
			if err != nil {
				message = err.Error()
				continue
			}
			if _, ok := findSourceEntry(ambience, entryID); !ok {
				message = "指定した音源IDがありません"
				continue
			}
			renderSourceGroups(output, ambience, "変更先")
			groupID, err := readSourceManagerID(reader, output, nil, "変更先の番号 > ")
			if err != nil {
				message = err.Error()
				continue
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if err := moveSourceURL(candidateAmbience, entryID, groupID); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源の使用場面を変更しました"

		case "d", "delete":
			entryID, err := readSourceManagerID(reader, output, parts[1:], "削除するID > ")
			if err != nil {
				message = err.Error()
				continue
			}
			entry, ok := findSourceEntry(ambience, entryID)
			if !ok {
				message = "指定した音源IDがありません"
				continue
			}
			confirm, err := readSourceManagerLine(reader, output, fmt.Sprintf("%s を削除しますか? [y/N] > ", truncate(entry.URL, 42)))
			if err != nil {
				return err
			}
			if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
				message = "削除を取り消しました"
				continue
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if _, err := deleteSourceURL(candidateAmbience, entryID); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源を削除しました"

		case "p", "preview":
			entryID, err := readSourceManagerID(reader, output, parts[1:], "試聴するID > ")
			if err != nil {
				message = err.Error()
				continue
			}
			entry, ok := findSourceEntry(ambience, entryID)
			if !ok {
				message = "指定した音源IDがありません"
				continue
			}
			fmt.Fprintf(output, "\n%s試聴中%s  %s\nSpaceキーで停止します…\n",
				ansiAudio, ansiReset, truncate(entry.URL, 64))
			if err := preview(ambience.PlayerCommand, entry.URL, ambience.Volume); err != nil {
				message = "試聴エラー: " + err.Error()
			} else {
				message = "試聴が完了しました"
			}

		case "q", "quit", "exit":
			return nil
		default:
			message = "操作は a / e / m / d / p / q のいずれかを指定してください"
		}
	}
}

func runSourceManagerCUI(path string, cfg *Config, output io.Writer, preview sourcePreviewFunc, console *sourceManagerConsole) error {
	selectedID := 1
	message := ""
	for {
		ambience, err := sourceConfigPointer(cfg)
		if err != nil {
			return err
		}
		entryCount := len(collectSourceEntries(ambience))
		selectedID = clampSourceSelection(selectedID, entryCount)
		renderSourceManager(output, path, ambience, message, selectedID, true)
		message = ""

		key, err := readImmediateKey(console.input)
		if err != nil {
			return err
		}
		switch key {
		case sourceKeyUp:
			selectedID = moveSourceSelection(selectedID, -1, entryCount)
		case sourceKeyDown:
			selectedID = moveSourceSelection(selectedID, 1, entryCount)

		case "a":
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			var groupID int
			var url string
			err = console.readLines(func(reader *bufio.Reader) error {
				renderSourceGroups(output, candidateAmbience, "追加先")
				var readErr error
				groupID, readErr = readSourceManagerID(reader, output, nil, "追加先の番号 > ")
				if readErr != nil {
					return readErr
				}
				url, readErr = readSourceManagerLine(reader, output, "URL > ")
				return readErr
			})
			if err != nil {
				message = err.Error()
				continue
			}
			if err := addSourceURL(candidateAmbience, groupID, url); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			if entry, ok := findSourceEntryByGroupAndURL(candidateAmbience, groupID, strings.TrimSpace(url)); ok {
				selectedID = entry.ID
			}
			message = "音源を登録しました"

		case "e":
			entry, ok := findSourceEntry(ambience, selectedID)
			if !ok {
				message = "編集する音源がありません"
				continue
			}
			var url string
			err := console.readLines(func(reader *bufio.Reader) error {
				fmt.Fprintf(output, "\n現在のURL: %s\n", entry.URL)
				var readErr error
				url, readErr = readSourceManagerLine(reader, output, "新しいURL > ")
				return readErr
			})
			if err != nil {
				message = err.Error()
				continue
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if err := editSourceURL(candidateAmbience, selectedID, url); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源URLを修正しました"

		case "m":
			entry, ok := findSourceEntry(ambience, selectedID)
			if !ok {
				message = "移動する音源がありません"
				continue
			}
			var groupID int
			err := console.readLines(func(reader *bufio.Reader) error {
				renderSourceGroups(output, ambience, "変更先")
				var readErr error
				groupID, readErr = readSourceManagerID(reader, output, nil, "変更先の番号 > ")
				return readErr
			})
			if err != nil {
				message = err.Error()
				continue
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if err := moveSourceURL(candidateAmbience, selectedID, groupID); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			if moved, ok := findSourceEntryByGroupAndURL(candidateAmbience, groupID, entry.URL); ok {
				selectedID = moved.ID
			}
			message = "音源の使用場面を変更しました"

		case "d":
			entry, ok := findSourceEntry(ambience, selectedID)
			if !ok {
				message = "削除する音源がありません"
				continue
			}
			fmt.Fprintf(output, "\n%s を削除しますか? [y/n] ", truncate(entry.URL, 42))
			confirmed, err := readSourceDeleteConfirmation(func() (string, error) {
				return readImmediateKey(console.input)
			})
			if err != nil {
				message = err.Error()
				continue
			}
			if !confirmed {
				message = "削除を取り消しました"
				continue
			}
			candidate, err := cloneConfig(*cfg)
			if err != nil {
				return err
			}
			candidateAmbience, err := sourceConfigPointer(&candidate)
			if err != nil {
				return err
			}
			if _, err := deleteSourceURL(candidateAmbience, selectedID); err != nil {
				message = err.Error()
				continue
			}
			if err := saveSourceConfig(path, candidate); err != nil {
				message = err.Error()
				continue
			}
			*cfg = candidate
			message = "音源を削除しました"

		case "p", sourceKeyEnter:
			entry, ok := findSourceEntry(ambience, selectedID)
			if !ok {
				message = "試聴する音源がありません"
				continue
			}
			fmt.Fprintf(output, "\n%s試聴中%s  %s\nSpaceキーで停止します…\n",
				ansiAudio, ansiReset, truncate(entry.URL, 64))
			if err := preview(ambience.PlayerCommand, entry.URL, ambience.Volume); err != nil {
				message = "試聴エラー: " + err.Error()
			} else {
				message = "試聴が完了しました"
			}

		case "q":
			return nil
		}
	}
}

func clampSourceSelection(selectedID, entryCount int) int {
	if entryCount == 0 {
		return 0
	}
	if selectedID < 1 {
		return 1
	}
	if selectedID > entryCount {
		return entryCount
	}
	return selectedID
}

func moveSourceSelection(selectedID, delta, entryCount int) int {
	if entryCount == 0 {
		return 0
	}
	selectedID = clampSourceSelection(selectedID, entryCount)
	return (selectedID-1+delta%entryCount+entryCount)%entryCount + 1
}

func readSourceDeleteConfirmation(readKey func() (string, error)) (bool, error) {
	for {
		key, err := readKey()
		if err != nil {
			return false, err
		}
		switch key {
		case "y":
			return true, nil
		case "n", sourceKeyEscape:
			return false, nil
		}
	}
}

func renderSourceManager(output io.Writer, path string, cfg *AmbienceConfig, message string, selectedID int, interactive bool) {
	fmt.Fprint(output, "\x1b[2J\x1b[H")
	fmt.Fprintf(output, "%sTOMATONE%s  音源管理\n", ansiBold, ansiReset)
	fmt.Fprintf(output, "%s設定: %s%s\n\n", ansiSubtle, path, ansiReset)

	entries := collectSourceEntries(cfg)
	if len(entries) == 0 {
		fmt.Fprintln(output, "  音源は登録されていません")
	} else {
		fmt.Fprintln(output, "    ID  使用場面              URL")
		fmt.Fprintln(output, "    ──  ──────────────────  ────────────────────────────────────────────────")
		for _, entry := range entries {
			marker := " "
			if entry.ID == selectedID {
				marker = ansiAudio + ">" + ansiReset
			}
			fmt.Fprintf(output, " %s  %2d  %s  %s\n", marker, entry.ID, sourceManagerCell(entry.GroupName, 18), truncate(entry.URL, 50))
		}
	}

	fmt.Fprintln(output)
	if interactive {
		fmt.Fprintln(output, " ↑/↓ select   [Enter/p] preview   [e] edit")
		fmt.Fprintln(output, " [a] add      [m] move            [d] delete   [q] quit")
	} else {
		fmt.Fprintln(output, " a add   e edit   m move   d delete   p preview (SPACE to stop)   q quit")
	}
	if message != "" {
		fmt.Fprintf(output, "%s%s%s\n", ansiAudio, message, ansiReset)
	}
}

func sourceManagerCell(value string, width int) string {
	value = truncate(value, width)
	padding := width - displayWidth(value)
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}

func renderSourceGroups(output io.Writer, cfg *AmbienceConfig, heading string) {
	fmt.Fprintf(output, "\n%s:\n", heading)
	for index, group := range collectSourceGroups(cfg) {
		fmt.Fprintf(output, " %d  %s (%d件)\n", index+1, group.Name, len(*group.URLs))
	}
}

func readSourceManagerLine(reader *bufio.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}
	return line, nil
}

func readSourceManagerID(reader *bufio.Reader, output io.Writer, args []string, prompt string) (int, error) {
	value := ""
	if len(args) > 0 {
		value = args[0]
	} else {
		var err error
		value, err = readSourceManagerLine(reader, output, prompt)
		if err != nil {
			return 0, err
		}
	}
	id, err := strconv.Atoi(value)
	if err != nil || id < 1 {
		return 0, errors.New("番号は1以上の整数で指定してください")
	}
	return id, nil
}

func sourceConfigPointer(cfg *Config) (*AmbienceConfig, error) {
	if cfg.Ambience != nil && cfg.YouTube != nil {
		return nil, errors.New("ambience と旧形式の youtube は同時に指定できません")
	}
	if cfg.Ambience != nil {
		return cfg.Ambience, nil
	}
	if cfg.YouTube != nil {
		return cfg.YouTube, nil
	}
	return nil, errors.New("ambience 設定がありません")
}

func collectSourceGroups(cfg *AmbienceConfig) []sourceGroup {
	groups := []sourceGroup{
		{Name: "デフォルト", URLs: &cfg.URLs},
		{Name: "集中", URLs: &cfg.FocusURLs},
		{Name: "休憩", URLs: &cfg.BreakURLs},
	}
	for index := range cfg.TimeRules {
		rule := &cfg.TimeRules[index]
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			name = fmt.Sprintf("時間帯 %s-%s", rule.Start, rule.End)
		}
		groups = append(groups,
			sourceGroup{Name: name + " / 共通", URLs: &rule.URLs},
			sourceGroup{Name: name + " / 集中", URLs: &rule.FocusURLs},
			sourceGroup{Name: name + " / 休憩", URLs: &rule.BreakURLs},
		)
	}
	return groups
}

func collectSourceEntries(cfg *AmbienceConfig) []sourceEntry {
	entries := make([]sourceEntry, 0)
	for groupIndex, group := range collectSourceGroups(cfg) {
		for listIndex, url := range *group.URLs {
			entries = append(entries, sourceEntry{
				ID:        len(entries) + 1,
				GroupID:   groupIndex + 1,
				GroupName: group.Name,
				ListIndex: listIndex,
				URL:       url,
			})
		}
	}
	return entries
}

func findSourceEntry(cfg *AmbienceConfig, id int) (sourceEntry, bool) {
	entries := collectSourceEntries(cfg)
	if id < 1 || id > len(entries) {
		return sourceEntry{}, false
	}
	return entries[id-1], true
}

func findSourceEntryByGroupAndURL(cfg *AmbienceConfig, groupID int, url string) (sourceEntry, bool) {
	for _, entry := range collectSourceEntries(cfg) {
		if entry.GroupID == groupID && entry.URL == url {
			return entry, true
		}
	}
	return sourceEntry{}, false
}

func addSourceURL(cfg *AmbienceConfig, groupID int, rawURL string) error {
	groups := collectSourceGroups(cfg)
	if groupID < 1 || groupID > len(groups) {
		return errors.New("指定した追加先がありません")
	}
	url, err := normalizedSourceURL(rawURL)
	if err != nil {
		return err
	}
	for _, existing := range *groups[groupID-1].URLs {
		if existing == url {
			return errors.New("同じURLが既に登録されています")
		}
	}
	*groups[groupID-1].URLs = append(*groups[groupID-1].URLs, url)
	return nil
}

func editSourceURL(cfg *AmbienceConfig, entryID int, rawURL string) error {
	entry, ok := findSourceEntry(cfg, entryID)
	if !ok {
		return errors.New("指定した音源IDがありません")
	}
	url, err := normalizedSourceURL(rawURL)
	if err != nil {
		return err
	}
	group := collectSourceGroups(cfg)[entry.GroupID-1]
	for index, existing := range *group.URLs {
		if index != entry.ListIndex && existing == url {
			return errors.New("同じURLが既に登録されています")
		}
	}
	(*group.URLs)[entry.ListIndex] = url
	return nil
}

func moveSourceURL(cfg *AmbienceConfig, entryID, destinationGroupID int) error {
	entry, ok := findSourceEntry(cfg, entryID)
	if !ok {
		return errors.New("指定した音源IDがありません")
	}
	groups := collectSourceGroups(cfg)
	if destinationGroupID < 1 || destinationGroupID > len(groups) {
		return errors.New("指定した変更先がありません")
	}
	if entry.GroupID == destinationGroupID {
		return errors.New("現在と同じ使用場面です")
	}

	destination := groups[destinationGroupID-1]
	for _, existing := range *destination.URLs {
		if existing == entry.URL {
			return errors.New("変更先に同じURLが既に登録されています")
		}
	}

	source := groups[entry.GroupID-1]
	sourceURLs := *source.URLs
	*source.URLs = append(sourceURLs[:entry.ListIndex], sourceURLs[entry.ListIndex+1:]...)
	*destination.URLs = append(*destination.URLs, entry.URL)
	return nil
}

func deleteSourceURL(cfg *AmbienceConfig, entryID int) (string, error) {
	entry, ok := findSourceEntry(cfg, entryID)
	if !ok {
		return "", errors.New("指定した音源IDがありません")
	}
	group := collectSourceGroups(cfg)[entry.GroupID-1]
	urls := *group.URLs
	*group.URLs = append(urls[:entry.ListIndex], urls[entry.ListIndex+1:]...)
	return entry.URL, nil
}

func normalizedSourceURL(rawURL string) (string, error) {
	url := strings.TrimSpace(rawURL)
	if err := validateAudioURL(url); err != nil {
		return "", err
	}
	return url, nil
}

func cloneConfig(cfg Config) (Config, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return Config{}, err
	}
	var cloned Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		return Config{}, err
	}
	return cloned, nil
}

func saveSourceConfig(path string, cfg Config) error {
	if _, err := validateConfig(cfg); err != nil {
		return err
	}
	if err := writeConfig(path, cfg, true); err != nil {
		return fmt.Errorf("設定を保存できません: %w", err)
	}
	return nil
}

func previewSource(playerCommand, rawURL string, volume int) error {
	playerPath, err := resolvePlayerCommand(playerCommand)
	if err != nil {
		return fmt.Errorf("%s が見つかりません", playerCommand)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args := buildSourcePreviewArgs(resolveMediaURL(rawURL), volume)
	cmd := exec.CommandContext(ctx, playerPath, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return err
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	restoreInput, immediate, err := enableImmediateInput(os.Stdin)
	if err != nil {
		cancel()
		<-waitCh
		return err
	}
	if !immediate {
		cancel()
		<-waitCh
		return errors.New("試聴はSpaceキーの即時入力が利用できるターミナルで実行してください")
	}
	defer restoreInput()

	if err := waitForPreviewStop(func() (string, error) { return readImmediateKey(os.Stdin) }); err != nil {
		cancel()
		<-waitCh
		return err
	}

	select {
	case err := <-waitCh:
		if err != nil {
			detail := lastOutputLine(output.String(), 120)
			if detail != "" {
				return errors.New(detail)
			}
			return err
		}
		return nil
	default:
		cancel()
		<-waitCh
		return nil
	}
}

func waitForPreviewStop(readKey func() (string, error)) error {
	for {
		key, err := readKey()
		if err != nil {
			return err
		}
		if key == " " {
			return nil
		}
	}
}

func buildSourcePreviewArgs(url string, volume int) []string {
	args := []string{
		"--no-config",
		"--no-video",
		"--force-window=no",
		"--input-terminal=no",
		"--volume=" + strconv.Itoa(volume),
		"--msg-level=all=error",
	}
	if isYouTubeURL(url) {
		args = append(args, "--ytdl=yes")
	} else {
		args = append(args, "--ytdl=no")
	}
	return append(args, url)
}
