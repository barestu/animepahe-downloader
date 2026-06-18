// Package app orchestrates the end-to-end flow: resolve a working base URL,
// search, pick episodes/quality (interactively or from flags), then download.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/barestu/animepahe-downloader/internal/animepahe"
	"github.com/barestu/animepahe-downloader/internal/client"
	"github.com/barestu/animepahe-downloader/internal/config"
	"github.com/barestu/animepahe-downloader/internal/download"
	"github.com/barestu/animepahe-downloader/internal/ui"
)

// Options holds resolved CLI flags.
type Options struct {
	Query     string
	Episodes  string // "", "all", "1", "1-12", "1,3,5"
	Quality   string // "", "min", "max", "1080"
	Audio     string // "", "jpn", "eng"
	OutputDir string
	Export    bool
	Resume    bool
	Verbose   bool // show raw ffmpeg log instead of a progress bar
}

// Run executes the flow. cfg.BaseURLs is tried in order until one answers.
func Run(ctx context.Context, cfg config.Config, opt Options) error {
	c, err := client.New(cfg.UserAgent)
	if err != nil {
		return err
	}
	for _, base := range cfg.BaseURLs {
		_ = c.SetCookies(base, cfg.Cookie)
	}

	interactive := opt.Query == ""

	// Probe for a working base URL (and clear any Cloudflare challenge) before
	// prompting for a search term, so the user doesn't type a query only to hit
	// a 403 wall. The probe prints per-base progress, so the run isn't silent.
	api, err := resolveAPI(c, cfg.BaseURLs)
	// On a Cloudflare challenge during an interactive run, let the user paste a
	// cf_clearance cookie + User-Agent. Apply them to the live client (no
	// rebuild), persist to the config file for future runs, and retry once.
	if err != nil && interactive && strings.Contains(err.Error(), "403") {
		cookie, ua, perr := ui.AskCloudflare(cfg.UserAgent)
		if perr != nil {
			return perr
		}
		if cookie != "" {
			cfg.Cookie = cookie
			if ua != "" {
				cfg.UserAgent = ua
			}
			c.SetUserAgent(cfg.UserAgent)
			for _, base := range cfg.BaseURLs {
				_ = c.SetCookies(base, cfg.Cookie)
			}
			if api, err = resolveAPI(c, cfg.BaseURLs); err == nil {
				if serr := config.Save(cfg); serr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", serr)
				} else {
					fmt.Fprintf(os.Stderr, "saved cf_clearance + user-agent to %s\n", config.ConfigPath())
				}
			}
		}
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "using base url: %s\n", api.BaseURL())

	query := opt.Query
	if query == "" {
		query, err = ui.AskInput("Search anime:", "")
		if err != nil {
			return err
		}
	}

	results, err := api.Search(query)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no results for %q", query)
	}

	var anime animepahe.Anime
	if interactive {
		anime, err = ui.SelectAnime(results)
		if err != nil {
			return err
		}
	} else {
		anime = results[0]
		fmt.Fprintf(os.Stderr, "selected: %s\n", anime.Title)
	}

	episodes, err := api.Episodes(anime.Session)
	if err != nil {
		return err
	}
	if len(episodes) == 0 {
		return fmt.Errorf("no episodes found for %s", anime.Title)
	}

	spec := opt.Episodes
	if spec == "" {
		if interactive {
			spec, err = ui.AskEpisodeRange()
			if err != nil {
				return err
			}
		} else {
			spec = "all"
		}
	}
	chosen, err := selectEpisodes(episodes, spec)
	if err != nil {
		return err
	}

	outDir := opt.OutputDir
	if outDir == "" {
		outDir = cfg.OutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// Quality is chosen once (interactively from the first episode) and reused.
	var pinned *animepahe.Quality
	for _, ep := range chosen {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		qualities, err := api.Links(anime.Session, ep.Session)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ep %g: links error: %v\n", ep.Episode, err)
			continue
		}
		if len(qualities) == 0 {
			fmt.Fprintf(os.Stderr, "ep %g: no qualities\n", ep.Episode)
			continue
		}

		q, err := pickQuality(qualities, opt, interactive, &pinned)
		if err != nil {
			return err
		}

		name := fmt.Sprintf("%s - %s - %sp.mp4", sanitize(anime.Title), episodeLabel(ep), q.Resolution)
		outPath := filepath.Join(outDir, name)

		if opt.Export {
			r, err := download.Resolve(c, q, api.BaseURL())
			if err != nil {
				fmt.Fprintf(os.Stderr, "ep %g: %v\n", ep.Episode, err)
				continue
			}
			fmt.Printf("%s\t%s\n", name, r.URL)
			continue
		}

		fmt.Fprintf(os.Stderr, "ep %g -> %s [%sp %s]\n", ep.Episode, name, q.Resolution, q.Audio)
		if err := withRetry(ctx, 3, func() error {
			return download.Episode(ctx, c, q, api.BaseURL(), outPath, cfg.UserAgent, opt.Resume, opt.Verbose)
		}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(os.Stderr, "ep %g: download failed: %v\n", ep.Episode, err)
		}
	}
	return nil
}

// resolveAPI returns an API bound to the first base URL that responds to a
// search probe — this transparently handles AnimePahe's domain rotation.
func resolveAPI(c *client.Client, bases []string) (*animepahe.API, error) {
	var lastErr error
	challenged := false
	fmt.Fprintln(os.Stderr, "resolving working base url...")
	for _, base := range bases {
		fmt.Fprintf(os.Stderr, "  probing %s ... ", base)
		api := animepahe.New(c, base)
		_, err := api.Search("a")
		if err == nil {
			fmt.Fprintln(os.Stderr, "ok")
			return api, nil
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		lastErr = err
		if strings.Contains(err.Error(), "403") {
			challenged = true
		}
	}
	if challenged {
		return nil, fmt.Errorf(`Cloudflare challenge blocked the request (HTTP 403).
Provide a cf_clearance cookie + matching User-Agent from a browser that has
visited the site:
  1. Open the site in Chrome, pass the "Just a moment..." check.
  2. DevTools > Application > Cookies, copy the cf_clearance value.
  3. DevTools > Network, copy the request User-Agent.
  4. Re-run with:  --cookie "cf_clearance=VALUE" --user-agent "UA"
     (or set ANIMEPAHE_COOKIE / ANIMEPAHE_USER_AGENT, or save them in %s)
last error: %w`, config.ConfigPath(), lastErr)
	}
	return nil, fmt.Errorf("no working base url (tried %v): %w", bases, lastErr)
}

func pickQuality(qs []animepahe.Quality, opt Options, interactive bool, pinned **animepahe.Quality) (animepahe.Quality, error) {
	// Filter by audio if requested.
	pool := qs
	if opt.Audio != "" {
		var f []animepahe.Quality
		for _, q := range qs {
			if strings.EqualFold(q.Audio, opt.Audio) {
				f = append(f, q)
			}
		}
		if len(f) > 0 {
			pool = f
		}
	}

	switch {
	case opt.Quality == "min":
		return extreme(pool, false), nil
	case opt.Quality == "max":
		return extreme(pool, true), nil
	case opt.Quality != "":
		for _, q := range pool {
			if q.Resolution == strings.TrimSuffix(opt.Quality, "p") {
				return q, nil
			}
		}
		return extreme(pool, true), nil // requested res absent -> best available
	}

	// Interactive: ask once, then reuse the same resolution/audio for the rest.
	if *pinned != nil {
		for _, q := range pool {
			if q.Resolution == (*pinned).Resolution && strings.EqualFold(q.Audio, (*pinned).Audio) {
				return q, nil
			}
		}
		return extreme(pool, true), nil
	}
	if interactive {
		q, err := ui.SelectQuality(pool)
		if err != nil {
			return animepahe.Quality{}, err
		}
		*pinned = &q
		return q, nil
	}
	return extreme(pool, true), nil
}

func extreme(qs []animepahe.Quality, max bool) animepahe.Quality {
	best := qs[0]
	for _, q := range qs[1:] {
		a, _ := strconv.Atoi(q.Resolution)
		b, _ := strconv.Atoi(best.Resolution)
		if (max && a > b) || (!max && a < b) {
			best = q
		}
	}
	return best
}

func withRetry(ctx context.Context, attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// ctrl+c cancelled the run — stop, don't retry.
		if ctx.Err() != nil {
			return err
		}
		// A missing ffmpeg won't fix itself — fail fast instead of retrying.
		if strings.Contains(err.Error(), "ffmpeg not found") {
			return err
		}
		if i < attempts-1 {
			wait := time.Duration(1<<i) * time.Second
			fmt.Fprintf(os.Stderr, "retry in %s (%v)\n", wait, err)
			time.Sleep(wait)
		}
	}
	return err
}

func episodeLabel(ep animepahe.Episode) string {
	if ep.Episode == float64(int(ep.Episode)) {
		return fmt.Sprintf("E%02d", int(ep.Episode))
	}
	return fmt.Sprintf("E%g", ep.Episode)
}

func sanitize(s string) string {
	repl := strings.NewReplacer("/", "_", "\\", "_", ":", "-", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	return strings.TrimSpace(repl.Replace(s))
}
