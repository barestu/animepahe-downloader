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
	"strings"

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
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once; avoid cobra's duplicate "Error:" line
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

	root.AddCommand(configCmd())

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

// configCmd builds the `apahe config` subcommand tree: set/show/path. These
// persist user preferences to the JSON config file without clobbering an
// auto-saved cf_clearance cookie.
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or persist configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist one config key (output-dir, base-url, user-agent)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "saved %s to %s\n", args[0], config.ConfigPath())
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the effective configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load("")
			fmt.Printf("base-url:    %s\n", strings.Join(cfg.BaseURLs, ", "))
			fmt.Printf("output-dir:  %s\n", cfg.OutputDir)
			fmt.Printf("user-agent:  %s\n", cfg.UserAgent)
			fmt.Printf("concurrency: %d\n", cfg.Concurrency)
			cookie := cfg.Cookie
			if cookie == "" {
				cookie = "(none)"
			} else {
				cookie = "(set)"
			}
			fmt.Printf("cookie:      %s\n", cookie)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(config.ConfigPath())
			return nil
		},
	})

	return cmd
}
