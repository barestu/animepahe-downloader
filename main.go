// Command animepahe-downloader searches AnimePahe and downloads episodes.
//
// Run bare for an interactive session, or pass flags to script it:
//
//	apahe -s "one piece" -e 1-12 -q 1080 -a jpn -o ./dl
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/barestu/animepahe-downloader/internal/app"
	"github.com/barestu/animepahe-downloader/internal/config"
	"github.com/spf13/cobra"
)

// version is overwritten at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// Cancel the root context on ctrl+c so an in-flight download (and the whole
	// run) stops cleanly instead of erroring out per-episode.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var (
		baseURL   string
		cookie    string
		userAgent string
		opt       app.Options
	)

	root := &cobra.Command{
		Use:     "apahe",
		Short:   "Search and download anime from AnimePahe",
		Version: version,
		Long: "Search and download anime from AnimePahe.\n\n" +
			"Run with no flags for an interactive session. Base URLs rotate; override\n" +
			"with --base-url, ANIMEPAHE_BASE_URL, or ~/.config/animepahe-dl/config.json.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load(baseURL)
			if cookie != "" {
				cfg.Cookie = cookie
			}
			if userAgent != "" {
				cfg.UserAgent = userAgent
			}
			return app.Run(cmd.Context(), cfg, opt)
		},
	}

	f := root.Flags()
	f.StringVarP(&opt.Query, "search", "s", "", "anime to search (omit for interactive prompt)")
	f.StringVarP(&opt.Episodes, "episodes", "e", "", "episodes: 1, 1-12, 1,3,5-8, or all")
	f.StringVarP(&opt.Quality, "quality", "q", "", "quality: min, max, or a resolution like 1080")
	f.StringVarP(&opt.Audio, "audio", "a", "", "audio language: jpn or eng")
	f.StringVarP(&opt.OutputDir, "output", "o", "", "output directory (default: current dir)")
	f.StringVar(&baseURL, "base-url", "", "comma-separated AnimePahe base URL(s) to use")
	f.StringVar(&cookie, "cookie", "", "raw Cookie header (e.g. \"cf_clearance=...\") to pass Cloudflare challenge")
	f.StringVar(&userAgent, "user-agent", "", "User-Agent to send (must match the browser that made the cookie)")
	f.BoolVar(&opt.Export, "export", false, "print resolved download links without downloading")
	f.BoolVar(&opt.Resume, "resume", false, "resume partial downloads (direct mp4 only)")
	f.BoolVar(&opt.Verbose, "debug", false, "show raw ffmpeg log instead of a progress bar")

	if err := root.ExecuteContext(ctx); err != nil {
		// ctrl+c at a prompt returns terminal.InterruptErr; ctrl+c mid-download
		// cancels ctx. Either way exit 130 (SIGINT) silently, no "error:" line.
		if errors.Is(err, terminal.InterruptErr) || ctx.Err() != nil {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
