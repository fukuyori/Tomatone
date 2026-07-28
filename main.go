package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tomatone:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("tomatone", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultPath, "設定ファイルのパス")
	showVersion := flags.Bool("version", false, "バージョンを表示")
	checkConfig := flags.Bool("check-config", false, "設定ファイルを検証して終了")
	checkURLs := flags.Bool("check-urls", false, "登録URLを実際に再生確認して終了")
	checkTimeout := flags.Duration("check-timeout", 30*time.Second, "URL検査の1件あたりのタイムアウト")
	var checkURLValues URLListFlag
	flags.Var(&checkURLValues, "check-url", "指定URLを実際に再生確認して終了（複数指定可）")
	flags.Usage = func() { printUsage(flags) }
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("Tomatone %s\n", version)
		return nil
	}
	if *checkConfig || *checkURLs || len(checkURLValues) > 0 {
		if len(flags.Args()) > 0 {
			return errors.New("検査オプションとサブコマンドは同時に指定できません")
		}
		if *checkURLs && len(checkURLValues) > 0 {
			return errors.New("--check-urls と --check-url は同時に指定できません")
		}
		_, cfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		fmt.Printf("CONFIG: OK  %s\n", *configPath)
		if *checkURLs {
			return checkConfiguredURLs(cfg.Ambience, *checkTimeout)
		}
		if len(checkURLValues) > 0 {
			return checkURLTargets(
				cfg.Ambience.PlayerCommand,
				explicitURLCheckTargets(checkURLValues),
				*checkTimeout)
		}
		return nil
	}

	remainingArgs := flags.Args()
	if len(remainingArgs) > 0 {
		switch remainingArgs[0] {
		case "config":
			return runConfigCommand(*configPath, remainingArgs[1:])
		case "chime":
			return runChimeCommand(*configPath, remainingArgs[1:])
		case "help":
			printUsage(flags)
			return nil
		default:
			return fmt.Errorf("不明なコマンドです: %s", remainingArgs[0])
		}
	}

	created, err := ensureConfig(*configPath)
	if err != nil {
		return err
	}
	_, cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr, "設定ファイルを作成しました: %s\n", *configPath)
	}
	return runTimer(*configPath, cfg)
}

func runChimeCommand(path string, args []string) error {
	command := "test"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "test":
		_, runtimeCfg, err := loadConfig(path)
		if err != nil {
			return err
		}
		vol := runtimeCfg.Chime.Volume
		fmt.Printf("チャイム音量テストを開始します (設定音量: %d%%)...\n", vol)
		if vol <= 0 {
			fmt.Println("※ 現在 chime.volume が 0% に指定されているため消音（ミュート）設定です。")
			return nil
		}
		fmt.Println("1. 休憩入りチャイム (Focus -> Break)")
		PlayChime(ChimeToBreak, vol)
		time.Sleep(1800 * time.Millisecond)
		fmt.Println("2. 集中開始チャイム (Break -> Focus)")
		PlayChime(ChimeToFocus, vol)
		time.Sleep(1500 * time.Millisecond)
		fmt.Println("テスト再生が完了しました。")
		return nil
	default:
		return fmt.Errorf("不明なchimeコマンドです: %s (例: tomatone chime test)", command)
	}
}

func runConfigCommand(path string, args []string) error {
	command := "show"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "path":
		fmt.Println(path)
		return nil
	case "init":
		if len(args) > 1 {
			return errors.New("config init に追加の引数は指定できません")
		}
		if err := writeConfig(path, defaultConfig(), false); err != nil {
			return err
		}
		fmt.Printf("設定ファイルを作成しました: %s\n", path)
		return nil
	case "show":
		if len(args) > 1 {
			return errors.New("config show に追加の引数は指定できません")
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
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "validate":
		_, _, err := loadConfig(path)
		if err != nil {
			return err
		}
		fmt.Printf("設定は有効です: %s\n", path)
		return nil
	default:
		return fmt.Errorf("不明なconfigコマンドです: %s", command)
	}
}

func runTimer(configPath string, cfg RuntimeConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	restoreInput, immediateInput, err := enableImmediateInput(os.Stdin)
	if err != nil {
		return err
	}

	timer := NewPomodoro(cfg)
	player := NewAmbientPlayer(cfg.Ambience)
	go player.Run(ctx)

	commands := make(chan string)
	go readCommands(ctx, os.Stdin, commands, immediateInput)

	ui := NewUI(os.Stdout, configPath, immediateInput)
	ui.Enter()
	defer func() {
		ui.Close()
		restoreInput()
		fmt.Println("Tomatoneを終了しました。")
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastTick := time.Now()
	ui.Render(timer, player.Snapshot())

	for {
		select {
		case <-ctx.Done():
			return nil
		case command, ok := <-commands:
			if !ok {
				stop()
				continue
			}
			cmd := strings.ToLower(command)
			if cmd != " " {
				cmd = strings.TrimSpace(cmd)
			}
			switch cmd {
			case "p", "pause", " ":
				timer.TogglePause()
				if timer.Running() {
					ui.SetMessage("タイマーを再開しました")
				} else {
					ui.SetMessage("タイマーを一時停止しました（アンビエンスは再生を続けます）")
				}
			case "c", "chime":
				PlayChime(ChimeToBreak, cfg.Chime.Volume)
				if cfg.Chime.Volume > 0 {
					ui.SetMessage(fmt.Sprintf("チャイム音をテスト再生しました (音量: %d%%)", cfg.Chime.Volume))
				} else {
					ui.SetMessage("チャイム音量は現在 0% (ミュート) です")
				}
			case "s", "skip":
				timer.Skip()
				player.SetPhase(timer.Phase())
				ui.SetMessage("次のフェーズへスキップしました")
			case "r", "reset":
				timer.Reset()
				ui.SetMessage("現在のフェーズをリセットしました")
			case "n", "next":
				player.Next()
				ui.SetMessage("次のアンビエンスへ切り替えています")
			case "+", "=":
				if _, ok := player.AdjustVolume(true); !ok {
					ui.SetMessage("再生開始後に音量を変更できます")
				} else {
					ui.SetMessage("")
				}
			case "-", "_":
				if _, ok := player.AdjustVolume(false); !ok {
					ui.SetMessage("再生開始後に音量を変更できます")
				} else {
					ui.SetMessage("")
				}
			case "q", "quit", "exit":
				stop()
				continue
			case "":
				ui.SetMessage("")
			default:
				ui.SetMessage("不明な操作です: " + command)
			}
			lastTick = time.Now()
			ui.Render(timer, player.Snapshot())
		case now := <-ticker.C:
			prevPhase := timer.Phase()
			if timer.Tick(now.Sub(lastTick)) {
				player.SetPhase(timer.Phase())
				if prevPhase == PhaseFocus {
					PlayChime(ChimeToBreak, cfg.Chime.Volume)
					ui.SetMessage("集中時間が終了しました！休憩に入りましょう ☕")
				} else {
					PlayChime(ChimeToFocus, cfg.Chime.Volume)
					ui.SetMessage("休憩時間が終了しました！集中を開始しましょう 🍅")
				}
			}
			lastTick = now
			ui.Render(timer, player.Snapshot())
		}
	}
}

func printUsage(flags *flag.FlagSet) {
	fmt.Fprintln(flags.Output(), `Tomatone - アンビエンス付きポモドーロタイマー

使い方:
  tomatone [--config PATH]
  tomatone [--config PATH] chime test
  tomatone [--config PATH] config init
  tomatone [--config PATH] config show
  tomatone [--config PATH] config path
  tomatone [--config PATH] config validate
  tomatone [--config PATH] --check-config
  tomatone [--config PATH] --check-url URL [--check-timeout 30s]
  tomatone [--config PATH] --check-urls [--check-timeout 30s]
  tomatone --version

設定:
  初回起動時に設定ファイルを自動作成します。
  再生には mpv が必要です。YouTubeを使う場合は yt-dlp も必要です。

オプション:`)
	flags.PrintDefaults()
}
