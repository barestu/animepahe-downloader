package download

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
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

// renderProgress reads ffmpeg's -progress stream and drives a progress bar.
// ffmpeg emits blocks of key=value lines ending in "progress=continue|end".
func renderProgress(r interface{ Read([]byte) (int, error) }, totalSeconds float64) {
	var bar *progressbar.ProgressBar
	if totalSeconds > 0 {
		bar = progressbar.NewOptions(1000,
			progressbar.OptionSetDescription("downloading"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionClearOnFinish(),
		)
	} else {
		bar = progressbar.NewOptions(-1,
			progressbar.OptionSetDescription("downloading"),
			progressbar.OptionSetWriter(os.Stderr),
		)
	}

	var curSeconds float64
	var sizeBytes int64
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
			if totalSeconds > 0 {
				frac := curSeconds / totalSeconds
				if frac > 1 {
					frac = 1
				}
				_ = bar.Set(int(frac * 1000))
				est := int64(0)
				if frac > 0 {
					est = int64(float64(sizeBytes) / frac)
				}
				bar.Describe(fmt.Sprintf("downloading %s / ~%s", humanSize(sizeBytes), humanSize(est)))
			} else {
				_ = bar.Add64(0)
				bar.Describe(fmt.Sprintf("downloading %s", humanSize(sizeBytes)))
			}
			if val == "end" {
				_ = bar.Finish()
			}
		}
	}
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
