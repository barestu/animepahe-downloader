// Package animepahe wraps the AnimePahe endpoints:
//
//	search:   GET {base}/api?m=search&q={query}        (JSON)
//	episodes: GET {base}/api?m=release&id={session}...  (JSON, paginated)
//	links:    GET {base}/play/{anime}/{episode}         (HTML, scraped — no JSON links API)
package animepahe

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/barestu/animepahe-downloader/internal/client"
)

// API talks to a single resolved base URL.
type API struct {
	client  *client.Client
	baseURL string
}

// New returns an API bound to baseURL (no trailing slash).
func New(c *client.Client, baseURL string) *API {
	return &API{client: c, baseURL: baseURL}
}

// BaseURL returns the bound base URL (used as Referer for kwik requests).
func (a *API) BaseURL() string { return a.baseURL }

// apiHeaders are sent on every /api call. AnimePahe rejects API requests that
// don't look like they came from a browser on the site: it wants a Referer
// pointing at the base domain and an XHR marker.
func (a *API) apiHeaders() map[string]string {
	return map[string]string{
		"referer":          a.baseURL + "/",
		"x-requested-with": "XMLHttpRequest",
	}
}

// Warmup fetches the homepage so the cookie jar picks up any clearance /
// DDoS-Guard cookie before the first API call. Errors are non-fatal.
func (a *API) Warmup() {
	_, _, _ = a.client.GetBytes(a.baseURL+"/", map[string]string{
		"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	})
}

// Anime is a search result.
type Anime struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Type     string  `json:"type"`
	Episodes int     `json:"episodes"`
	Status   string  `json:"status"`
	Season   string  `json:"season"`
	Year     int     `json:"year"`
	Score    float64 `json:"score"`
	Poster   string  `json:"poster"`
	Session  string  `json:"session"`
}

type searchResp struct {
	Total int     `json:"total"`
	Data  []Anime `json:"data"`
}

// Episode is one release entry.
type Episode struct {
	ID       int     `json:"id"`
	AnimeID  int     `json:"anime_id"`
	Episode  float64 `json:"episode"`
	Title    string  `json:"title"`
	Snapshot string  `json:"snapshot"`
	Audio    string  `json:"audio"`
	Duration string  `json:"duration"`
	Session  string  `json:"session"`
	Filler   int     `json:"filler"`
}

type releaseResp struct {
	Total       int       `json:"total"`
	PerPage     int       `json:"per_page"`
	CurrentPage int       `json:"current_page"`
	LastPage    int       `json:"last_page"`
	Data        []Episode `json:"data"`
}

// Quality is a single downloadable rendition for an episode.
type Quality struct {
	Resolution string // "360" / "720" / "1080"
	Audio      string // "jpn" / "eng"
	FileSize   int64
	Fansub     string
	KwikURL    string // https://kwik.cx/e/{id} embed page (HLS path)
	PaheWin    string // https://pahe.win/{token} redirect (direct mp4 path)
}

// Search returns anime matching query.
func (a *API) Search(query string) ([]Anime, error) {
	u := fmt.Sprintf("%s/api?m=search&q=%s", a.baseURL, url.QueryEscape(query))
	b, err := a.client.GetJSON(u, a.apiHeaders())
	if err != nil {
		return nil, err
	}
	var r searchResp
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("decode search: %w", err)
	}
	return r.Data, nil
}

// Episodes returns all episodes for an anime session, walking every page.
func (a *API) Episodes(session string) ([]Episode, error) {
	var all []Episode
	page := 1
	for {
		u := fmt.Sprintf("%s/api?m=release&id=%s&sort=episode_asc&page=%d", a.baseURL, url.QueryEscape(session), page)
		b, err := a.client.GetJSON(u, a.apiHeaders())
		if err != nil {
			return nil, err
		}
		var r releaseResp
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("decode release page %d: %w", page, err)
		}
		all = append(all, r.Data...)
		if r.LastPage <= page || len(r.Data) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// Links returns available qualities for an episode by scraping the play page
// (AnimePahe has no JSON links endpoint — the page itself carries the kwik
// embed buttons and the pahe.win download anchors).
func (a *API) Links(animeSession, episodeSession string) ([]Quality, error) {
	u := fmt.Sprintf("%s/play/%s/%s", a.baseURL, url.PathEscape(animeSession), url.PathEscape(episodeSession))
	b, err := a.client.GetJSON(u, a.apiHeaders())
	if err != nil {
		return nil, err
	}
	html := string(b)

	// Direct mp4 download anchors, keyed by "fansub|resolution".
	type pahe struct {
		url  string
		size int64
	}
	directs := map[string]pahe{}
	for _, m := range paheAnchorRe.FindAllStringSubmatch(html, -1) {
		fansub, res := parseDownloadLabel(m[2])
		if res == "" {
			continue
		}
		directs[fansub+"|"+res] = pahe{url: m[1], size: parseSize(m[2])}
	}

	// kwik embed buttons carry the structured metadata.
	var out []Quality
	for _, m := range kwikButtonRe.FindAllStringSubmatch(html, -1) {
		q := Quality{
			KwikURL:    m[1],
			Fansub:     m[2],
			Resolution: m[3],
			Audio:      m[4],
		}
		if d, ok := directs[q.Fansub+"|"+q.Resolution]; ok {
			q.PaheWin = d.url
			q.FileSize = d.size
		}
		out = append(out, q)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no download links found on play page (layout may have changed)")
	}
	return out, nil
}

var (
	// <button ... data-src="kwik" data-fansub=".." data-resolution=".." data-audio="..">
	kwikButtonRe = regexp.MustCompile(`data-src="(https://kwik\.[^"]+)"\s+data-fansub="([^"]*)"\s+data-resolution="([^"]*)"\s+data-audio="([^"]*)"`)
	// <a href="pahe.win/.." ...>SubsPlease &middot; 1080p (172MB)</a>
	paheAnchorRe = regexp.MustCompile(`<a[^>]+href="(https://pahe\.win/[^"]+)"[^>]*>([^<]+)</a>`)
	resInLabelRe = regexp.MustCompile(`(\d+)p`)
	sizeRe       = regexp.MustCompile(`\(([\d.]+)\s*([KMG]B)\)`)
)

// parseDownloadLabel extracts fansub and resolution from a label like
// "SubsPlease &middot; 1080p (172MB)".
func parseDownloadLabel(label string) (fansub, resolution string) {
	if rm := resInLabelRe.FindStringSubmatch(label); rm != nil {
		resolution = rm[1]
	}
	if i := strings.Index(label, "&middot;"); i >= 0 {
		fansub = strings.TrimSpace(label[:i])
	} else if i := strings.Index(label, "·"); i >= 0 {
		fansub = strings.TrimSpace(label[:i])
	}
	return fansub, resolution
}

// parseSize converts a "(172MB)" / "(1.5GB)" label into bytes.
func parseSize(label string) int64 {
	m := sizeRe.FindStringSubmatch(label)
	if m == nil {
		return 0
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	switch m[2] {
	case "KB":
		return int64(n * 1024)
	case "MB":
		return int64(n * 1024 * 1024)
	case "GB":
		return int64(n * 1024 * 1024 * 1024)
	}
	return 0
}
