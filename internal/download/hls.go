package download

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// HLS downloads an m3u8 playlist to outPath (mp4) using ffmpeg. ffmpeg pulls and
// concatenates the segments itself; we pass the kwik referer + UA so the CDN
// serves them. Requires ffmpeg on PATH.
//
// totalSeconds is the playlist duration (0 if unknown) used to render a percent
// progress bar. When verbose is true the raw ffmpeg log is shown instead.
func HLS(ctx context.Context, m3u8URL, referer, userAgent, outPath string, totalSeconds float64, verbose bool) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found on PATH (required for HLS streams)")
	}
	headers := fmt.Sprintf("Referer: %s\r\nUser-Agent: %s\r\n", referer, userAgent)
	args := []string{"-y", "-headers", headers, "-i", m3u8URL, "-c", "copy", "-bsf:a", "aac_adtstoasc"}

	if verbose {
		args = append(args, outPath)
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg: %w", err)
		}
		return nil
	}

	// Quiet mode: machine-readable progress on stdout, errors on stderr.
	args = append(args, "-loglevel", "error", "-progress", "pipe:1", "-nostats", outPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	renderProgress(stdout, totalSeconds)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}

// renderProgress reads ffmpeg's -progress stream and prints a single
// self-overwriting status line (percent + downloaded size + speed). ffmpeg emits
// blocks of key=value lines ending in "progress=continue|end". Download speed is
// derived from the change in total_size over wall-clock time between blocks
// (ffmpeg's own speed= key is encode-x-realtime, not bytes/sec).
func renderProgress(r interface{ Read([]byte) (int, error) }, totalSeconds float64) {
	var curSeconds float64
	var sizeBytes int64
	var lastSize int64
	start := time.Now()
	lastTime := start
	var speed float64 // bytes/sec

	const clearLine = "\r\033[K"

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us", "out_time_ms": // microseconds (mislabeled in some builds)
			if us, err := strconv.ParseInt(val, 10, 64); err == nil {
				curSeconds = float64(us) / 1_000_000
			}
		case "total_size":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				sizeBytes = n
			}
		case "progress":
			now := time.Now()
			if dt := now.Sub(lastTime).Seconds(); dt > 0 {
				speed = float64(sizeBytes-lastSize) / dt
			}
			lastSize, lastTime = sizeBytes, now

			if totalSeconds > 0 {
				frac := curSeconds / totalSeconds
				if frac > 1 {
					frac = 1
				}
				pct := frac * 100
				est := int64(0)        // projected total file size
				eta := time.Duration(0) // time left
				if frac > 0 {
					est = int64(float64(sizeBytes) / frac)
					eta = time.Duration((totalSeconds-curSeconds)/curSeconds*now.Sub(start).Seconds()) * time.Second
				}
				fmt.Fprintf(os.Stderr, "%sdownloading %.0f%% %s / ~%s %s/s ETA %s",
					clearLine, pct, humanSize(sizeBytes), humanSize(est), humanSize(int64(speed)), fmtDuration(eta))
			} else {
				fmt.Fprintf(os.Stderr, "%sdownloading %s %s/s", clearLine, humanSize(sizeBytes), humanSize(int64(speed)))
			}
			if val == "end" {
				fmt.Fprint(os.Stderr, clearLine)
			}
		}
	}
	// A scanner error just means we stopped drawing progress early; the actual
	// download success/failure is decided by cmd.Wait() in HLS.
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%sprogress read error: %v\n", clearLine, err)
	}
}

// fmtDuration renders a duration as M:SS (or H:MM:SS past an hour).
func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func humanSize(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGT"[exp])
}
