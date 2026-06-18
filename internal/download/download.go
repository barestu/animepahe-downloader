package download

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/barestu/animepahe-downloader/internal/animepahe"
	"github.com/barestu/animepahe-downloader/internal/client"
	"github.com/barestu/animepahe-downloader/internal/kwik"
)

// Resolved is the outcome of resolving a quality to a concrete URL.
type Resolved struct {
	URL string
	HLS bool // true => URL is an m3u8 to feed ffmpeg
}

// Resolve turns a Quality into a concrete media URL, preferring a direct mp4
// and falling back to an HLS playlist. referer is the AnimePahe base URL.
func Resolve(c *client.Client, q animepahe.Quality, referer string) (Resolved, error) {
	var errs []string
	// Direct path: pahe.win redirect or kwik /f/ download form -> mp4.
	for _, page := range []string{q.PaheWin, q.KwikURL} {
		if page == "" {
			continue
		}
		url, err := kwik.DirectLink(c, page, referer)
		if err == nil && url != "" {
			return Resolved{URL: url}, nil
		}
		if err != nil {
			errs = append(errs, "direct: "+err.Error())
		}
	}
	// Fallback: HLS m3u8 via the kwik embed page.
	if q.KwikURL != "" {
		url, err := kwik.M3U8(c, q.KwikURL, referer)
		if err == nil && url != "" {
			return Resolved{URL: url, HLS: true}, nil
		}
		if err != nil {
			errs = append(errs, "hls: "+err.Error())
		}
	}
	return Resolved{}, fmt.Errorf("could not resolve %sp: %s", q.Resolution, strings.Join(errs, "; "))
}

// Episode resolves q and downloads it to outPath. outPath should end in .mp4.
// verbose shows the raw ffmpeg log (HLS) instead of a progress bar. ctx cancels
// an in-flight download (e.g. on ctrl+c).
func Episode(ctx context.Context, c *client.Client, q animepahe.Quality, referer, outPath, userAgent string, resume, verbose bool) error {
	r, err := Resolve(c, q, referer)
	if err != nil {
		return err
	}
	if r.HLS {
		if !strings.HasSuffix(strings.ToLower(outPath), ".mp4") {
			outPath += ".mp4"
		}
		// The stream host (owocdn) gates the playlist/segments on a kwik Referer,
		// not the AnimePahe one.
		const kwikRef = "https://kwik.cx/"
		dur := playlistDuration(c, r.URL, kwikRef)
		return HLS(ctx, r.URL, kwikRef, userAgent, outPath, dur, verbose)
	}
	return Direct(ctx, c, r.URL, referer, outPath, resume)
}

// playlistDuration fetches an m3u8 and sums its #EXTINF durations (seconds).
// Returns 0 if it can't be determined (e.g. a master playlist).
func playlistDuration(c *client.Client, m3u8URL, referer string) float64 {
	body, status, err := c.GetBytes(m3u8URL, map[string]string{"referer": referer})
	if err != nil || status != 200 {
		return 0
	}
	var total float64
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		// "#EXTINF:6.381," -> "6.381"
		v := strings.SplitN(strings.TrimPrefix(line, "#EXTINF:"), ",", 2)[0]
		if d, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			total += d
		}
	}
	return total
}
