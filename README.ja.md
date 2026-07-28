# Tomatone

[English](README.md) | 日本語

バージョン: **0.1.0**

リポジトリ: [github.com/fukuyori/Tomatone](https://github.com/fukuyori/Tomatone)

Tomatone は、YouTubeやネットラジオのアンビエントサウンドを流しながら使うCUIポモドーロタイマーです。毎秒更新されるタイマーと、再生中の番組・動画タイトル、音量、再生時間を一つの固定ターミナル画面に表示します。

## 必要なもの

- Go 1.24以降（ソースからビルドする場合）
- [mpv](https://mpv.io/)（音声再生と再生状況の取得）
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)（YouTubeを使う場合のみ）

`mpv` が `PATH` にあることを確認してください。YouTubeも利用する場合は `yt-dlp` も必要です。Tomatoneは動画を表示せず、音声だけを再生します。

## ビルドと起動

```powershell
go build -buildvcs=false -o tomatone.exe .
.\tomatone.exe
```

Gitリポジトリ内では通常の `go build -o tomatone.exe .` でもビルドできます。まだGit管理されていないディレクトリでVCS情報の検出エラーが出る場合だけ `-buildvcs=false` を付けてください。

初回起動時に設定ファイルが作成されます。場所は次のコマンドで確認できます。

```powershell
.\tomatone.exe config path
.\tomatone.exe config show
```

任意の場所に設定を作る場合:

```powershell
.\tomatone.exe --config .\config.json config init
.\tomatone.exe --config .\config.json
```

## 設定

作成された `config.json` をエディターで開き、時間とアンビエンスURLを編集します。リポジトリ内の [`config.example.json`](config.example.json) と、ネットラジオ専用の [`config.radio.example.json`](config.radio.example.json) も見本として利用できます。

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
        "name": "夜間モード",
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

時間にはGoの期間表記を使います。例: `30s`, `25m`, `1h30m`。

- `chime.volume`: 切り替えチャイム通知音の音量（0〜100、デフォルト: 80）。`0` に指定するとチャイム音を消音（ミュート）できます。
- `ambience.urls`: 基本の音源URLのリストです。YouTube、ネットラジオの直接配信URL、HTTP(S)の `.m3u`、`.pls`、`.m3u8` プレイリストを混在できます。
- `focus_urls`: 集中時間（Focusフェーズ）に優先再生されるURLリストです。
- `break_urls`: 休憩時間（Breakフェーズ）に優先再生されるURLリストです。
- `time_rules`: 時間帯（例: `06:00`〜`18:00` や `21:00`〜`06:00`）に応じたBGM切り替えルールを設定できます。
  - 優先度: `time_rules` (時間帯一致) ➔ `focus_urls`/`break_urls` (フェーズ一致) ➔ `urls` (デフォルト)
- `shuffle`: `true`なら、再生開始時、曲の終了時、フェーズに応じたURLリストの切り替え時、`n`操作時に、使用するURLをリストからランダムに選びます。候補が2件以上ある場合は、同じURLの連続再生を避けます。`false`なら記載順に再生します。
- `volume`: mpvの音量（0〜100）です。
- `player_command`: 通常は `mpv`。PATHにない場合は実行ファイルのフルパスを指定できます。

以前の `"youtube"` 設定名も後方互換として読み込めます。その中へネットラジオURLを追加することもできます。新しい設定では `"ambience"` を使用してください。`"ambience"` と `"youtube"` は同時には指定できません。

上記のネットラジオ例は [SomaFM Groove Saladの公式PLS](https://somafm.com/groovesalad/directstreamlinks.html) です。任意の局が公開しているHTTP(S)ストリームURLへ置き換えられます。

編集後は設定の検証やチャイム音量のテスト再生ができます。

```powershell
.\tomatone.exe config validate
.\tomatone.exe chime test
```

非対話オプションでも検査できます。

```powershell
# JSON、時間、音量、URL形式などの設定検査
.\tomatone.exe --check-config

# 設定検査に加え、全URLをmpvで無音・短時間再生
.\tomatone.exe --check-urls

# 設定へ登録する前に1件だけ確認
.\tomatone.exe --check-url "https://www.nts.live/infinite-mixtapes/slow-focus"

# 接続が遅い場合は1件あたりの制限時間を変更
.\tomatone.exe --check-urls --check-timeout 60s
```

`--check-urls`は全登録URLを検査します。`--check-url`は未登録URLを個別に検査でき、複数回指定することもできます。YouTubeは`yt-dlp`経由、ネットラジオはmpvへ直接接続します。重複URLは一度だけ検査し、1件でも失敗すると終了コード1を返します。

NTS Slow FocusはWebページURLをそのまま登録できます。Tomatoneが確認済みの直接ストリームへ自動変換します。

```json
"urls": [
  "https://www.nts.live/infinite-mixtapes/slow-focus"
]
```

## おすすめラジオチャンネル

以下は集中時間と休憩時間に使いやすいチャンネルの例です。記載URLは2026年7月28日にTomatoneの`--check-url`で再生確認しています。ラジオ局側の都合でURLや配信内容が変更されることがあります。

### 集中時間向け

| 種類 | チャンネル | 内容 | 登録URL |
|---|---|---|---|
| 自然音 | [HearMe Nature](https://hearme.fm/radio/nature/) | 水音、鳥の声、雨、森林音 | `https://radio.hearme.fm:8950/stream` |
| 自然音 | [HearMe Ocean Sounds](https://hearme.fm/radio/ocean-sounds/) | 波、潮風、海岸の環境音 | `https://radio.hearme.fm:8162/stream` |
| アンビエント | [NTS Slow Focus](https://www.nts.live/infinite-mixtapes/slow-focus) | ビートの少ないアンビエント、ドローン、ラーガ | `https://www.nts.live/infinite-mixtapes/slow-focus` |
| アンビエント | [SomaFM Drone Zone](https://somafm.com/dronezone/directstreamlinks.html) | 最小限のビートと空間的なドローン | `https://somafm.com/dronezone.pls` |
| クラシック | [Radio Swiss Classic](https://www.radioswissclassic.ch/en/reception/internet) | クラシック専門、MP3 128kbps | `https://stream.srg-ssr.ch/srgssr/rsc_de/mp3/128` |
| ジャズ | [Radio Swiss Jazz](https://www.radioswissjazz.ch/en/reception/internet) | ジャズ、スウィング、ソウル、ブルース | `https://stream.srg-ssr.ch/srgssr/rsj/mp3/128` |
| ジャズ | [SomaFM Sonic Universe](https://somafm.com/sonicuniverse/directstreamlinks.html) | 現代ジャズ、アヴァンギャルド寄り | `https://somafm.com/sonicuniverse.pls` |
| カフェ | [SomaFM Bossa Beyond](https://somafm.com/bossa/directstreamlinks.html) | ボサノヴァ、サンバ、ブラジル音楽 | `https://somafm.com/bossa.pls` |
| カフェ | [SomaFM Illinois Street Lounge](https://somafm.com/illstreet/directstreamlinks.html) | ラウンジ、エキゾチカ、ヴィンテージ音楽 | `https://somafm.com/illstreet.pls` |
| カフェ／チル | [SomaFM Groove Salad](https://somafm.com/groovesalad/directstreamlinks.html) | ダウンテンポ、アンビエント・ビート | `https://somafm.com/groovesalad.pls` |

### 休憩時間向け

| 種類 | チャンネル | 内容 | 登録URL |
|---|---|---|---|
| 軽いポップス | [Radio Swiss Pop](https://www.radioswisspop.ch/en/reception/internet) | 聴きやすい幅広いポップス | `https://stream.srg-ssr.ch/srgssr/rsp/mp3/128` |
| インディーポップ | [SomaFM Indie Pop Rocks!](https://somafm.com/indiepop/directstreamlinks.html) | 軽快なインディーポップ、ロック | `https://somafm.com/indiepop.pls` |
| ソフトロック | [SomaFM Left Coast 70s](https://somafm.com/seventies/directstreamlinks.html) | 穏やかな1970年代ウェストコースト・ロック | `https://somafm.com/seventies.pls` |
| エレクトロポップ | [SomaFM PopTron](https://somafm.com/poptron/directstreamlinks.html) | 明るいエレクトロポップ、軽いダンスロック | `https://somafm.com/poptron.pls` |

設定例:

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

`shuffle`を`true`にすると、再生するチャンネルを選択されたURLリストからランダムに選びます。候補が2件以上ある場合は、同じチャンネルの連続再生を避けます。SomaFMの配信URLは、同局の案内に従って個人利用の範囲で使用してください。設定後は`.\tomatone.exe --check-urls`で、現在も各URLへ接続できるか確認できます。

## 操作

起動するとターミナルがCUI画面へ切り替わり、タイマーとアンビエンス再生が始まります。Windowsでは各キーを直接押して操作でき、Enterは不要です。終了すると元のターミナル画面と入力モードへ戻ります。

| 入力 | 動作 |
|---|---|
| `p` / `Space` | タイマーの一時停止・再開。アンビエンスは流れ続けます |
| `c` | チャイム通知音のテスト再生（設定音量の確認） |
| `s` | 現在のフェーズをスキップ |
| `r` | 現在のフェーズの残り時間をリセット |
| `n` | 次のアンビエンスURLへ進む |
| `+` / `-` | 再生中の音量を5%ずつ変更 |
| `q` | 終了 |

集中時間が終わると短い休憩へ移り、設定したセット数を完了すると長い休憩へ移ります。フェーズが変わってもアンビエンスは停止しません。CUIで変更した音量は次のリンクにも引き継がれますが、設定ファイルには保存されません。

標準入力をパイプから渡した場合や即時入力を利用できない環境では、入力後にEnterを押す行入力へ自動的に切り替わります。

## バージョン

```powershell
.\tomatone.exe --version
```

## テスト

```powershell
go test ./...
```
