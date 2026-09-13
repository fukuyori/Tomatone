# Tomatone

Version: **0.2.0**

A CUI Pomodoro timer with YouTube and internet-radio ambience.

- [English documentation](docs/README.md)
- [日本語ドキュメント](docs/README.ja.md)
- [Changelog](docs/CHANGELOG.md)
- [Configuration examples](examples/)

Manage ambience sources in the terminal:

```powershell
.\tomatone.exe sources
```

## Build

```powershell
go build -o tomatone.exe ./cmd/tomatone
.\tomatone.exe
```

## Test

```powershell
go test ./...
```
