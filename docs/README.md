# Tomatone

English | [日本語](README.ja.md)

Version: **0.1.1**

Repository: [github.com/fukuyori/Tomatone](https://github.com/fukuyori/Tomatone)

Tomatone is a CUI Pomodoro timer that plays ambient audio from YouTube and internet radio. Its fixed terminal dashboard updates every second and displays the timer, current programme or video title, volume, and playback time.

## Requirements

- Go 1.24 or later (when building from source)
- [mpv](https://mpv.io/) for audio playback and playback status
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) only when using YouTube

Make sure `mpv` is available on `PATH`. YouTube playback additionally requires `yt-dlp`. Tomatone plays audio only and does not open a video window.

## Build and run

```powershell
go build -buildvcs=false -o tomatone.exe ./cmd/tomatone
.\tomatone.exe
```

Inside a Git repository, the regular `go build -o tomatone.exe ./cmd/tomatone` command also works. Use `-buildvcs=false` only if Go reports a VCS metadata detection error in a directory that is not under Git.

Tomatone creates a configuration file on first launch. Use these commands to find and inspect it:

```powershell
.\tomatone.exe config path
.\tomatone.exe config show
```

To create and use a configuration file at a specific path:

```powershell
.\tomatone.exe --config .\config.json config init
.\tomatone.exe --config .\config.json
```

## Configuration

Open the generated `config.json` and edit the timer values and ambience URLs. This repository also includes [`config.example.json`](../examples/config.example.json) and the radio-oriented [`config.radio.example.json`](../examples/config.radio.example.json).

```json
{
  "timer": {
    "focus": "25m",
    "short_break": "5m",
    "long_break": "15m",
    "focus_sessions_before_long_break": 4,
    "auto_start": true
  },
  "chime": {
    "volume": 80
  },
  "ambience": {
    "urls": [
      "https://www.youtube.com/watch?v=DEFAULT_VIDEO_ID",
      "https://somafm.com/groovesalad.pls"
    ],
    "focus_urls": [
      "https://www.youtube.com/watch?v=FOCUS_BGM_ID"
    ],
    "break_urls": [
      "https://www.youtube.com/watch?v=BREAK_BGM_ID"
    ],
    "time_rules": [
      {
        "name": "Night mode",
        "start": "21:00",
        "end": "06:00",
        "focus_urls": [
          "https://www.youtube.com/watch?v=NIGHT_FOCUS_ID"
        ],
        "break_urls": [
          "https://www.youtube.com/watch?v=NIGHT_BREAK_ID"
        ]
      }
    ],
    "shuffle": true,
    "volume": 45,
    "player_command": "mpv"
  }
}
```

Timer values use Go duration syntax, such as `30s`, `25m`, or `1h30m`.

- `chime.volume`: Transition chime volume from 0 to 100 (default: 80). Set it to `0` to mute the chime.
- `ambience.urls`: Default audio sources. YouTube URLs, direct internet-radio streams, and HTTP(S) `.m3u`, `.pls`, or `.m3u8` playlists can be mixed in one list.
- `focus_urls`: Sources preferred during Focus phases.
- `break_urls`: Sources preferred during short and long Break phases.
- `time_rules`: Source-selection rules for time ranges such as `06:00`–`18:00` or `21:00`–`06:00`.
  - Priority: matching `time_rules` → matching `focus_urls` or `break_urls` → default `urls`
- `shuffle`: When `true`, Tomatone randomly selects a URL at startup, after a track ends, when the phase-specific URL list changes, and when `n` is pressed. With two or more candidates, it avoids immediately repeating the same URL. When `false`, URLs play in the listed order.
- `volume`: mpv volume from 0 to 100.
- `player_command`: Normally `mpv`. If mpv is not on `PATH`, specify its full executable path.

The legacy `"youtube"` configuration key remains supported and may also contain internet-radio URLs. New configurations should use `"ambience"`. Do not specify `"ambience"` and `"youtube"` at the same time.

The radio URL in the example is SomaFM Groove Salad's [official PLS playlist](https://somafm.com/groovesalad/directstreamlinks.html). It may be replaced with any HTTP(S) stream URL published by a radio station.

After editing the configuration, validate it or test the chime volume:

```powershell
.\tomatone.exe config validate
.\tomatone.exe chime test
```

The same checks are available as non-interactive options:

```powershell
# Validate JSON, timer values, volume, and URL formats
.\tomatone.exe --check-config

# Validate the configuration and silently probe every registered URL with mpv
.\tomatone.exe --check-urls

# Probe one URL before adding it to the configuration
.\tomatone.exe --check-url "https://www.nts.live/infinite-mixtapes/slow-focus"

# Increase the per-URL timeout for slower connections
.\tomatone.exe --check-urls --check-timeout 60s
```

`--check-urls` probes every configured URL. `--check-url` probes an unregistered URL and may be specified multiple times. YouTube is resolved through `yt-dlp`; internet radio is connected directly through mpv. Duplicate URLs are checked once. The command exits with status 1 if any URL fails.

The NTS Slow Focus webpage URL can be registered directly. Tomatone automatically resolves it to the verified audio stream:

```json
"urls": [
  "https://www.nts.live/infinite-mixtapes/slow-focus"
]
```

## Recommended radio channels

The following channels work well for focus and break sessions. Every registration URL was probed with Tomatone's `--check-url` option on July 28, 2026. A station may change its URL or programming later.

### Focus

| Category | Channel | Content | Registration URL |
|---|---|---|---|
| Nature | [HearMe Nature](https://hearme.fm/radio/nature/) | Water, birdsong, rain, and forest sounds | `https://radio.hearme.fm:8950/stream` |
| Nature | [HearMe Ocean Sounds](https://hearme.fm/radio/ocean-sounds/) | Waves, sea breeze, and coastal ambience | `https://radio.hearme.fm:8162/stream` |
| Ambient | [NTS Slow Focus](https://www.nts.live/infinite-mixtapes/slow-focus) | Beatless ambient, drone, and ragas | `https://www.nts.live/infinite-mixtapes/slow-focus` |
| Ambient | [SomaFM Drone Zone](https://somafm.com/dronezone/directstreamlinks.html) | Atmospheric drone with minimal beats | `https://somafm.com/dronezone.pls` |
| Classical | [Radio Swiss Classic](https://www.radioswissclassic.ch/en/reception/internet) | Classical music, MP3 at 128 kbps | `https://stream.srg-ssr.ch/srgssr/rsc_de/mp3/128` |
| Jazz | [Radio Swiss Jazz](https://www.radioswissjazz.ch/en/reception/internet) | Jazz, swing, soul, and blues | `https://stream.srg-ssr.ch/srgssr/rsj/mp3/128` |
| Jazz | [SomaFM Sonic Universe](https://somafm.com/sonicuniverse/directstreamlinks.html) | Contemporary and avant-garde jazz | `https://somafm.com/sonicuniverse.pls` |
| Café | [SomaFM Bossa Beyond](https://somafm.com/bossa/directstreamlinks.html) | Bossa nova, samba, and Brazilian music | `https://somafm.com/bossa.pls` |
| Café | [SomaFM Illinois Street Lounge](https://somafm.com/illstreet/directstreamlinks.html) | Lounge, exotica, and vintage music | `https://somafm.com/illstreet.pls` |
| Café / chill | [SomaFM Groove Salad](https://somafm.com/groovesalad/directstreamlinks.html) | Downtempo and ambient beats | `https://somafm.com/groovesalad.pls` |

### Break

| Category | Channel | Content | Registration URL |
|---|---|---|---|
| Light pop | [Radio Swiss Pop](https://www.radioswisspop.ch/en/reception/internet) | Accessible pop from a broad range of eras | `https://stream.srg-ssr.ch/srgssr/rsp/mp3/128` |
| Indie pop | [SomaFM Indie Pop Rocks!](https://somafm.com/indiepop/directstreamlinks.html) | Upbeat indie pop and rock | `https://somafm.com/indiepop.pls` |
| Soft rock | [SomaFM Left Coast 70s](https://somafm.com/seventies/directstreamlinks.html) | Mellow 1970s West Coast rock | `https://somafm.com/seventies.pls` |
| Electropop | [SomaFM PopTron](https://somafm.com/poptron/directstreamlinks.html) | Bright electropop and light dance rock | `https://somafm.com/poptron.pls` |

Example:

```json
"ambience": {
  "urls": [],
  "focus_urls": [
    "https://radio.hearme.fm:8950/stream",
    "https://radio.hearme.fm:8162/stream",
    "https://www.nts.live/infinite-mixtapes/slow-focus",
    "https://somafm.com/dronezone.pls",
    "https://stream.srg-ssr.ch/srgssr/rsc_de/mp3/128",
    "https://stream.srg-ssr.ch/srgssr/rsj/mp3/128",
    "https://somafm.com/sonicuniverse.pls",
    "https://somafm.com/bossa.pls",
    "https://somafm.com/illstreet.pls",
    "https://somafm.com/groovesalad.pls"
  ],
  "break_urls": [
    "https://stream.srg-ssr.ch/srgssr/rsp/mp3/128",
    "https://somafm.com/indiepop.pls",
    "https://somafm.com/seventies.pls",
    "https://somafm.com/poptron.pls"
  ],
  "time_rules": [],
  "shuffle": true,
  "volume": 45,
  "player_command": "mpv"
}
```

With `shuffle` enabled, Tomatone selects a random channel from the active URL list and avoids immediately repeating a channel when alternatives exist. Use SomaFM stream URLs only for personal use as specified by the station. Run `.\tomatone.exe --check-urls` after configuring the channels to confirm that they remain reachable.

## Controls

At startup, Tomatone switches to its CUI dashboard and starts the timer and ambience player. On Windows, press each key directly without Enter. The original terminal display and input mode are restored when Tomatone exits.

| Input | Action |
|---|---|
| `p` / `Space` | Pause or resume the timer. Ambience keeps playing. |
| `c` | Test the transition chime at the configured volume. |
| `s` | Skip the current phase. |
| `r` | Reset the remaining time of the current phase. |
| `n` | Select the next ambience source. |
| `+` / `-` | Increase or decrease playback volume by 5%. |
| `q` | Quit. |

After a Focus phase, Tomatone enters a short break. After the configured number of Focus sessions, it enters a long break. Ambience continues across phase boundaries unless the active URL list changes. Volume adjustments made in the CUI carry over to the next source but are not saved to the configuration file.

If direct key input is unavailable, such as when standard input is piped, Tomatone automatically falls back to line input and requires Enter after each command.

## Version

```powershell
.\tomatone.exe --version
```

## Tests

```powershell
go test ./...
```
