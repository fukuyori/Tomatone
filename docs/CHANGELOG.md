# Changelog

All notable changes to Tomatone are documented in this file.

## [0.1.2] - 2026-08-15

### Added

- Add an arrow-key CUI source manager for registering, editing, moving between usage groups, deleting, and previewing ambience URLs until `Space` is pressed.

### Fixed

- Disable mpv terminal input during previews so Tomatone exclusively receives the preview stop key.
- Prefer the real `mpv.exe` over Scoop's `mpv.com` launcher so stopping playback terminates the managed player process.
- Accept delete confirmation with immediate `y`/`n` key input in the source manager.

## [0.1.1] - 2026-07-28

### Fixed

- Clear temporary CUI operation messages after three seconds instead of leaving them on screen indefinitely.
- Show playback state on the operation-message line after a temporary message disappears.

### Changed

- Organize the Go command under `cmd/tomatone`, detailed documentation under `docs`, and configuration samples under `examples`.

## [0.1.0] - 2026-07-28

### Added

- CUI Pomodoro dashboard with focus, short-break, and long-break phases.
- YouTube and internet-radio ambience playback through mpv.
- Separate source lists for focus and break phases, plus time-based source rules.
- Random source selection with immediate-repeat avoidance.
- Playback title, volume, elapsed time, and connection status display.
- Keyboard controls for timer actions, source switching, chime testing, and volume.
- Configuration and URL validation commands.
- Recommended focus and break radio-channel documentation.
