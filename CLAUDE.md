# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Go is installed at `$HOME/.local/go` and is **not on PATH** by default — prefix commands:

```sh
export PATH=$HOME/.local/go/bin:$PATH
go build -o apahe .                  # build single binary (or: make build)
go vet ./...
go test ./...                        # unit tests (only internal/kwik has them)
go test ./internal/kwik/ -run TestUnpack -v   # single test
```

ffmpeg must be on `PATH` to actually download (the working path is HLS). Without
it, only `--export` works.

## Architecture

CLI that searches AnimePahe and downloads episodes. Flow lives in `internal/app/app.go`
(`Run`), which wires the packages together:

1. **`internal/client`** — the Cloudflare-aware HTTP layer wrapping
   `bogdanfinn/tls-client`. **The TLS/JA3 profile is chosen from the User-Agent's
   browser family** (`Firefox_135` + no `sec-ch-ua` when the UA says Firefox, else
   `Chrome_120` + Chrome client-hints) — a mismatched UA/JA3 trips Cloudflare on
   cookieless hosts like kwik.cx. Holds *two* underlying clients on one shared
   cookie jar: redirect-following (normal) and non-following (`RawDoNoRedirect`,
   to read a 302 `Location`). `SetCookies` seeds a `cf_clearance` cookie;
   `SetUserAgent` swaps the UA without a rebuild (used after the live prompt).
2. **`internal/animepahe`** — search + release are JSON (`?m=search`,
   `?m=release` paginated). **There is no links API** (`?m=links` returns "not
   enough arguments"); `Links` scrapes the play page `/play/{anime}/{ep}` for
   kwik embed buttons (`data-src`/`data-resolution`/`data-audio`) and pahe.win
   download anchors. Every request sends `Referer` + `X-Requested-With`.
3. **`internal/kwik`** — resolves kwik.cx links. `unpack.go` reverses the
   Dean-Edwards `p,a,c,k,e,d` packed JS (radix can be 62; the unbaser handles
   base>36); the one unit-tested core. `resolve.go`: the kwik embed page has
   **multiple packed blocks — the `.m3u8` is in a later one, so scan them all**.
   `normalizeReferer` forces a trailing slash (kwik 403s on a slashless Referer).
4. **`internal/download`** — `Resolve` tries direct mp4 first then falls back to
   HLS. In practice the direct path (pahe.win → kwik.cx/f/) is itself
   CF-challenged, so HLS wins. `hls.go` shells to ffmpeg with
   **`Referer: https://kwik.cx/`** (the stream host owocdn gates on the kwik
   referer, not the AnimePahe one) and the UA; the playlist is AES-128 and ffmpeg
   fetches the key itself.
5. **`internal/app`** — `resolveAPI` probes each base URL (handles rotation) and
   prints per-base progress; the search prompt runs *before* the probe so an
   interactive run isn't silent. On a 403 challenge it prompts for cookie+UA via
   `ui.AskCloudflare`, applies them in place, retries, and on success persists
   them with `config.Save`. `episodes.go` parses `1,3,5-8`/`all`; quality is
   pinned once and reused; retry skips ffmpeg-missing (not transient).
6. **`internal/config`** — precedence highest-first: flag > env
   (`ANIMEPAHE_BASE_URL`/`ANIMEPAHE_COOKIE`/`ANIMEPAHE_USER_AGENT`) > JSON file
   (`~/.config/animepahe-dl/config.json`, written 0600) > defaults. `Set`
   persists one key (`output-dir`/`base-url`/`user-agent`) without clobbering an
   auto-saved cookie; `SaveCredentials` writes the harvested cookie+UA.
7. **`internal/upgrade`** — `apahe upgrade` subcommand. `Report` hits the GitHub
   latest-release API and prints whether a newer tag exists plus the
   `go install ...@latest` line. **Check-and-instruct only** — never
   self-replaces the binary, runs only when invoked explicitly.

`main.go` wires the cobra root plus two subcommand trees: `config` (set/show/path)
and `upgrade`. Version is injected via `-ldflags "-X main.version=..."`.

## Cloudflare constraint (important)

Base domains **rotate** (don't hardcode; `.org`/others 301 → `.pw`) and the site
runs Cloudflare's **managed JS challenge** (`cf-mitigated: challenge`).
TLS-spoofing alone **cannot** pass it. Workaround: a browser-harvested
`cf_clearance` cookie + matching UA (interactive prompt that auto-saves, or
`--cookie`/`--user-agent`/env/config). The **UA must match the browser that
produced the cookie** (cf_clearance is bound to IP+UA), and the client's JA3
profile auto-matches that UA's family (Chrome/Firefox). kwik.cx and the owocdn
stream host have their own referer gates (see points 3–4). A headless-browser
auto-harvest is the documented future upgrade, kept out to preserve the
dependency-free binary.
