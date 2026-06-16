# animepahe-downloader

A maintained Go rewrite of the unmaintained
[animepahe-cli](https://github.com/Danushka-Madushan/animepahe-cli). Search
AnimePahe, list episodes, and download them — single static binary, interactive
by default, scriptable via flags.

## Features

- Search anime and pick interactively, or drive everything with flags.
- Episode ranges and batch download (`1`, `1-12`, `1,3,5-8`, `all`).
- Quality / audio selection (`--quality 1080 --audio jpn`, or `min`/`max`).
- Downloads the HLS stream via ffmpeg (handles the AES-128 encrypted segments);
  attempts a direct mp4 first and falls back automatically. Shows a progress bar
  (percent, downloaded / estimated size, ETA) by default; `--debug` for the raw
  ffmpeg log.
- Configurable, rotating base URLs (the site changes domains often).
- Cloudflare-aware HTTP: TLS/JA3 fingerprint matched to the User-Agent, plus a
  `cf_clearance` cookie passthrough for the managed JS challenge.
- `--export` to print the resolved stream URL without downloading.

## Requirements

- **ffmpeg** on `PATH` — required to download (the working path is HLS). Without
  it, downloads fail with a clear message; `--export` still works.
- Go 1.21+ only to build from source.

## Build

```sh
go build -o apahe .      # or: make build
```

## Usage

Interactive:

```sh
./apahe
```

Scripted:

```sh
./apahe -s "one piece" -e 1-12 -q 1080 -a jpn -o ./downloads
./apahe -s "naruto" -e 1 --export        # print links only
```

| flag | meaning |
|------|---------|
| `-s, --search` | anime to search (omit → interactive prompt) |
| `-e, --episodes` | `1`, `1-12`, `1,3,5-8`, or `all` |
| `-q, --quality` | `min`, `max`, or a resolution like `1080` |
| `-a, --audio` | `jpn` or `eng` |
| `-o, --output` | output directory |
| `--base-url` | comma-separated base URL(s) |
| `--cookie` | raw Cookie header for the Cloudflare challenge |
| `--user-agent` | User-Agent (must match the cookie's browser) |
| `--export` | print the resolved stream URL, don't download |
| `--resume` | resume a partial direct download |
| `--debug` | show the raw ffmpeg log instead of the progress bar |

First interactive run prompts for the Cloudflare cookie (see below) and saves it;
after that a bare `./apahe` just asks for the search term.

## Configurable base URLs

The site rotates domains. Defaults: `https://animepahe.pw`, `https://animepahe.com`,
`https://animepahe.org`. The tool tries each until one answers. Override via:

- `--base-url "https://animepahe.pw,https://animepahe.com"`
- env `ANIMEPAHE_BASE_URL`
- `~/.config/animepahe-dl/config.json` (see below)

## Cloudflare

AnimePahe sits behind Cloudflare's **managed challenge** (the "Just a moment…"
page, header `cf-mitigated: challenge`). TLS spoofing alone cannot pass it — the
challenge needs JavaScript. The lightweight, single-binary workaround is to
reuse a `cf_clearance` cookie from a browser that already cleared the check:

1. Open the site in **Chrome or Firefox** and pass the "Just a moment…" page.
2. DevTools → Application/Storage → Cookies → copy the `cf_clearance` value.
3. DevTools → Network → copy the request `User-Agent`.
4. Provide the matching pair (cookie + its browser's UA).

In an **interactive run** you don't need flags — on the challenge the tool
prompts for the `cf_clearance` value and User-Agent, applies them immediately,
and **saves them to the config file** so later runs reuse them automatically.

Or pass them directly:

```sh
./apahe -s "naruto" \
  --cookie "cf_clearance=PASTE_VALUE" \
  --user-agent "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0"
```

The TLS fingerprint auto-matches the UA's browser family (Chrome or Firefox), so
the cookie works regardless of which of the two you harvested it from — but the
UA **must** match the browser that produced the cookie.

`cf_clearance` is bound to your IP + User-Agent and lasts ~30 min to a few hours;
refresh it when requests start 403ing again (re-run interactively and paste a new
value, or edit the config file).

> Direct-mp4 links (pahe.win → kwik.cx/f/) are themselves Cloudflare-challenged,
> so downloads currently use the HLS stream via ffmpeg. A headless-browser
> cookie auto-harvest is a possible future upgrade, left out to keep this a
> dependency-free single binary.

## Config file

`~/.config/animepahe-dl/config.json`:

```json
{
  "base_urls": ["https://animepahe.pw", "https://animepahe.com"],
  "output_dir": "/home/me/anime",
  "user_agent": "Mozilla/5.0 (...) Chrome/120.0.0.0 Safari/537.36",
  "cookie": "cf_clearance=...",
  "concurrency": 4
}
```

Precedence (highest first): flag > env > config file > built-in defaults.
