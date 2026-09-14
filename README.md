# Tomatone

Version: **0.2.1**

A CUI Pomodoro timer with YouTube and internet-radio ambience.

- [English documentation](docs/README.md)
- [日本語ドキュメント](docs/README.ja.md)
- [Changelog](docs/CHANGELOG.md)
- [Configuration examples](examples/)

Manage ambience sources in the terminal after building:

Windows:

```powershell
.\tomatone.exe sources
```

macOS / Linux:

```sh
./tomatone sources
```

## Requirements

- Go 1.24 or later
- [mpv](https://mpv.io/) for audio playback
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) only when using YouTube

`mpv` and, when needed, `yt-dlp` must be available on `PATH`.

## Install playback tools

### Windows (PowerShell with Scoop)

```powershell
scoop bucket add extras
scoop install extras/mpv

# Only when using YouTube
scoop install yt-dlp
```

### macOS (Homebrew)

```sh
brew install mpv

# Only when using YouTube
brew install yt-dlp
```

### Debian / Ubuntu

```sh
sudo apt update
sudo apt install mpv

# Only when using YouTube
sudo apt install yt-dlp
```

For other platforms and installation methods, see the [mpv installation guide](https://mpv.io/installation/) and the [yt-dlp installation guide](https://github.com/yt-dlp/yt-dlp/wiki/Installation).

Verify that the installed commands can be found:

```sh
mpv --version
yt-dlp --version # Only required for YouTube
```

## Build and run

### Windows

```powershell
go build -o tomatone.exe ./cmd/tomatone
.\tomatone.exe
```

### macOS / Linux

```sh
go build -o tomatone ./cmd/tomatone
./tomatone
```

## Test

```sh
go test ./...
```
