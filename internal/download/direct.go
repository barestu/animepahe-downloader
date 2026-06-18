// Package download fetches an episode to disk. It auto-selects between a direct
// mp4 download and an HLS (m3u8 + ffmpeg) download.
package download

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/barestu/animepahe-downloader/internal/client"
	http "github.com/bogdanfinn/fhttp"
	"github.com/schollz/progressbar/v3"
)

// Direct streams mediaURL to outPath with a progress bar. If resume is set and
// a partial file exists, it requests the remaining byte range and appends. ctx
// cancellation (ctrl+c) closes the response body to unblock the copy.
func Direct(ctx context.Context, c *client.Client, mediaURL, referer, outPath string, resume bool) error {
	extra := map[string]string{}
	if referer != "" {
		extra["referer"] = referer
	}

	var start int64
	if resume {
		if fi, err := os.Stat(outPath); err == nil && fi.Size() > 0 {
			start = fi.Size()
			extra["range"] = fmt.Sprintf("bytes=%d-", start)
		}
	}

	resp, err := c.Do(mediaURL, extra)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// The underlying client isn't ctx-aware; close the body on cancel so the
	// io.Copy below returns instead of blocking until the download finishes.
	stop := context.AfterFunc(ctx, func() { resp.Body.Close() })
	defer stop()

	switch resp.StatusCode {
	case http.StatusOK:
		start = 0 // server ignored range; restart
	case http.StatusPartialContent:
		// resuming
	default:
		return fmt.Errorf("download: unexpected status %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if start > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(outPath, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength + start
	bar := progressbar.DefaultBytes(total, "downloading")
	if _, err := io.Copy(io.MultiWriter(f, bar), resp.Body); err != nil {
		return fmt.Errorf("download: copy: %w", err)
	}
	return nil
}
