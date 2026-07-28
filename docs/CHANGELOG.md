# Changelog

All notable changes to Tomatone are documented in this file.

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
