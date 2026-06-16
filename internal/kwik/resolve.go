package kwik

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/barestu/animepahe-downloader/internal/client"
	http "github.com/bogdanfinn/fhttp"
)

var (
	// packed eval block on a kwik page.
	packedRe = regexp.MustCompile(`(?s)(eval\(function\(p,a,c,k,e,d\).*?\.split\('\|'\),0,\{\}\)\))`)
	// .m3u8 URL inside the unpacked HLS player source.
	m3u8Re = regexp.MustCompile(`https?://[^\s'"\\]+\.m3u8[^\s'"\\]*`)
	// download form on the kwik /f/ page (after unpack): action + _token.
	formActionRe = regexp.MustCompile(`<form[^>]+action="([^"]+)"`)
	tokenRe      = regexp.MustCompile(`name="_token"\s+value="([^"]+)"`)
	// pahe.win sometimes exposes a direct anchor.
	btnHrefRe = regexp.MustCompile(`<a[^>]+href="([^"]+)"[^>]*class="[^"]*btn[^"]*"`)
)

// M3U8 fetches a kwik embed/download page and extracts the HLS playlist URL.
// referer must be the AnimePahe base URL (kwik gates on it).
func M3U8(c *client.Client, kwikURL, referer string) (string, error) {
	body, err := fetchPage(c, kwikURL, referer)
	if err != nil {
		return "", err
	}
	// Try a direct match first, then unpack every packed block (the embed page
	// has several; the m3u8 lives in a later one, not the first).
	if u := m3u8Re.FindString(body); u != "" {
		return u, nil
	}
	for _, packed := range packedRe.FindAllString(body, -1) {
		src, err := Unpack(packed)
		if err != nil {
			continue
		}
		if u := m3u8Re.FindString(src); u != "" {
			return u, nil
		}
	}
	return "", fmt.Errorf("kwik: no m3u8 found in %s", kwikURL)
}

// DirectLink resolves the kwik /f/ (or pahe.win) page to a direct media URL by
// extracting the POST download form and submitting it. Returns the final file
// URL (after the redirect) without downloading it.
func DirectLink(c *client.Client, kwikURL, referer string) (string, error) {
	body, err := fetchPage(c, kwikURL, referer)
	if err != nil {
		return "", err
	}
	// Some shortener pages expose the link straight away.
	if m := btnHrefRe.FindStringSubmatch(body); m != nil && isMedia(m[1]) {
		return m[1], nil
	}
	// Unpack to reveal the hidden download form.
	if packed := packedRe.FindString(body); packed != "" {
		if src, err := Unpack(packed); err == nil {
			body = body + "\n" + src
		}
	}
	action := firstSubmatch(formActionRe, body)
	token := firstSubmatch(tokenRe, body)
	if action == "" || token == "" {
		return "", fmt.Errorf("kwik: download form not found in %s", kwikURL)
	}
	final, err := postForm(c, action, kwikURL, url.Values{"_token": {token}})
	if err != nil {
		return "", err
	}
	if final == "" {
		return "", fmt.Errorf("kwik: form POST did not redirect to a file")
	}
	return final, nil
}

func fetchPage(c *client.Client, pageURL, referer string) (string, error) {
	b, status, err := c.GetBytes(pageURL, map[string]string{"referer": normalizeReferer(referer)})
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("kwik: GET %s status %d", pageURL, status)
	}
	return string(b), nil
}

// postForm submits the download form and returns the Location of the redirect
// to the actual media file (kwik responds 302 to the CDN object).
func postForm(c *client.Client, action, referer string, form url.Values) (string, error) {
	req, err := http.NewRequest(http.MethodPost, action, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header = http.Header{
		"user-agent":   {c.UserAgent()},
		"content-type": {"application/x-www-form-urlencoded"},
		"referer":      {referer},
	}
	resp, err := c.RawDoNoRedirect(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if loc := resp.Header.Get("Location"); loc != "" {
		return loc, nil
	}
	return "", nil
}

// normalizeReferer ensures a trailing slash — kwik.cx returns 403 for a
// schemes-and-host Referer without it (e.g. "https://animepahe.pw").
func normalizeReferer(referer string) string {
	if referer == "" {
		return referer
	}
	return strings.TrimRight(referer, "/") + "/"
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func isMedia(u string) bool {
	u = strings.ToLower(u)
	return strings.Contains(u, ".mp4") || strings.Contains(u, ".mkv")
}
