// Package ui provides interactive selection prompts used when the tool is run
// without enough flags to proceed non-interactively.
package ui

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/barestu/animepahe-downloader/internal/animepahe"
)

// SelectAnime asks the user to pick one anime from search results.
func SelectAnime(results []animepahe.Anime) (animepahe.Anime, error) {
	opts := make([]string, len(results))
	for i, a := range results {
		opts[i] = fmt.Sprintf("%s (%s, %d) — %d eps [%s]", a.Title, a.Type, a.Year, a.Episodes, a.Status)
	}
	var idx int
	err := survey.AskOne(&survey.Select{
		Message:  "Select anime:",
		Options:  opts,
		PageSize: 12,
	}, &idx)
	if err != nil {
		return animepahe.Anime{}, err
	}
	return results[idx], nil
}

// SelectQuality asks the user to pick one quality/rendition.
func SelectQuality(qs []animepahe.Quality) (animepahe.Quality, error) {
	opts := make([]string, len(qs))
	for i, q := range qs {
		size := ""
		if q.FileSize > 0 {
			size = fmt.Sprintf(" ~%dMB", q.FileSize/(1024*1024))
		}
		opts[i] = fmt.Sprintf("%sp [%s]%s %s", q.Resolution, q.Audio, size, q.Fansub)
	}
	var idx int
	err := survey.AskOne(&survey.Select{
		Message: "Select quality:",
		Options: opts,
	}, &idx)
	if err != nil {
		return animepahe.Quality{}, err
	}
	return qs[idx], nil
}

// AskCloudflare prompts for a cf_clearance cookie value and a User-Agent to
// pass Cloudflare's managed challenge. Returns a ready Cookie header string
// ("cf_clearance=...") and the UA. An empty cookie means the user declined.
func AskCloudflare(defaultUA string) (cookie, userAgent string, err error) {
	fmt.Println("Cloudflare challenge detected. Paste a cf_clearance cookie + matching")
	fmt.Println("User-Agent from a browser that has cleared the \"Just a moment...\" page.")

	var value string
	if err = survey.AskOne(&survey.Input{
		Message: "cf_clearance value (blank to cancel):",
	}, &value); err != nil {
		return "", "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", nil
	}
	// Accept either a bare value or a full "cf_clearance=..." paste.
	value = strings.TrimPrefix(value, "cf_clearance=")

	if err = survey.AskOne(&survey.Input{
		Message: "User-Agent (must match the cookie's browser):",
		Default: defaultUA,
	}, &userAgent); err != nil {
		return "", "", err
	}
	return "cf_clearance=" + value, strings.TrimSpace(userAgent), nil
}

// AskInput prompts for a free-text value with an optional default.
func AskInput(message, def string) (string, error) {
	var ans string
	err := survey.AskOne(&survey.Input{Message: message, Default: def}, &ans)
	return ans, err
}

// AskEpisodeRange prompts for an episode selection like "1", "1-12" or "all".
func AskEpisodeRange() (string, error) {
	var ans string
	err := survey.AskOne(&survey.Input{
		Message: "Episodes (e.g. 1, 1-12, all):",
		Default: "all",
	}, &ans)
	return ans, err
}
