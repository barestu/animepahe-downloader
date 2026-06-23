// Package upgrade checks GitHub for a newer release and instructs the user how
// to install it. It is check-and-instruct only: it never replaces the running
// binary. The release query uses stdlib net/http (not the Cloudflare-aware
// tls-client) since api.github.com is not behind a challenge.
package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// latestReleaseURL is the GitHub API endpoint for the most recent release.
	latestReleaseURL = "https://api.github.com/repos/barestu/animepahe-downloader/releases/latest"
	// installCmd is the one-liner that installs the latest version from source.
	installCmd = "go install github.com/barestu/animepahe-downloader@latest"
)

// Release is the subset of the GitHub release payload we care about.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// LatestRelease fetches the latest release tag from GitHub with a short timeout.
func LatestRelease(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github release check returned %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, err
	}
	if rel.TagName == "" {
		return Release{}, fmt.Errorf("github release check returned an empty tag")
	}
	return rel, nil
}

// SameVersion reports whether two version strings are equal after stripping a
// leading "v" from each. It is plain inequality, not semver ordering: any
// difference is treated as "a newer release exists".
func SameVersion(a, b string) bool {
	return normalize(a) == normalize(b)
}

// normalize strips a single leading "v" and surrounding whitespace.
func normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// Report runs the check against the current version and writes a human-readable
// result to w. version=="dev" (built from source) skips the check.
func Report(ctx context.Context, w io.Writer, current string) error {
	if normalize(current) == "dev" {
		fmt.Fprintln(w, "built from source (version \"dev\"); skipping the release check.")
		fmt.Fprintf(w, "to install a released build: %s\n", installCmd)
		return nil
	}

	rel, err := LatestRelease(ctx)
	if err != nil {
		return err
	}

	if SameVersion(current, rel.TagName) {
		fmt.Fprintf(w, "up to date (%s).\n", rel.TagName)
		return nil
	}

	fmt.Fprintf(w, "a newer release is available: %s (you have %s)\n", rel.TagName, current)
	fmt.Fprintf(w, "release page: %s\n", rel.HTMLURL)
	fmt.Fprintf(w, "install with: %s\n", installCmd)
	fmt.Fprintln(w, "or download the archive for your OS/arch from the release page.")
	return nil
}
