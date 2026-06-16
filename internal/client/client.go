// Package client wraps bogdanfinn/tls-client to defeat AnimePahe's Cloudflare /
// DDoS-Guard protection. Those gates fingerprint the TLS ClientHello (JA3) and
// cross-check it against the User-Agent; a stock net/http client is rejected.
// We present a real Chrome TLS profile plus matching browser headers and keep a
// cookie jar so any clearance cookie issued on the first request is reused.
package client

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// Client is a Cloudflare-aware HTTP client. It keeps two underlying clients
// sharing one cookie jar: one follows redirects (normal fetches), one does not
// (so we can read a 302 Location, e.g. the kwik download form target).
type Client struct {
	http       tls_client.HttpClient
	noRedirect tls_client.HttpClient
	jar        tls_client.CookieJar
	userAgent  string
	firefox    bool
}

// New builds a client whose TLS fingerprint matches the User-Agent's browser
// family (a mismatched UA/JA3 trips Cloudflare on cookieless hosts like kwik.cx)
// and keeps a persistent cookie jar.
func New(userAgent string) (*Client, error) {
	firefox := strings.Contains(userAgent, "Firefox")
	profile := profiles.Chrome_120
	if firefox {
		profile = profiles.Firefox_135
	}
	jar := tls_client.NewCookieJar()
	base := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profile),
		tls_client.WithCookieJar(jar),
	}
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), base...)
	if err != nil {
		return nil, err
	}
	nr, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), append(base, tls_client.WithNotFollowRedirects())...)
	if err != nil {
		return nil, err
	}
	return &Client{http: c, noRedirect: nr, jar: jar, userAgent: userAgent, firefox: firefox}, nil
}

// SetCookies seeds the cookie jar for baseURL's host from a raw Cookie header
// string (e.g. "cf_clearance=abc; other=1"). Used to inject a cf_clearance
// cookie harvested from a real browser so we ride past the managed challenge.
func (c *Client) SetCookies(baseURL, cookieHeader string) error {
	if strings.TrimSpace(cookieHeader) == "" {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	var cookies []*http.Cookie
	for _, part := range strings.Split(cookieHeader, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{Name: kv[0], Value: kv[1], Domain: u.Hostname(), Path: "/"})
	}
	c.jar.SetCookies(u, cookies)
	return nil
}

// RawDo executes an arbitrary request (following redirects). Caller closes Body.
func (c *Client) RawDo(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}

// RawDoNoRedirect executes a request without following redirects, so the
// Location header of a 3xx is observable. Caller closes Body.
func (c *Client) RawDoNoRedirect(req *http.Request) (*http.Response, error) {
	return c.noRedirect.Do(req)
}

// Do issues a GET and returns the raw response. Caller must close Body.
// extra headers (e.g. referer, range) override the defaults.
func (c *Client) Do(url string, extra map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = http.Header{
		"user-agent":      {c.userAgent},
		"accept":          {"application/json, text/plain, */*"},
		"accept-language": {"en-US,en;q=0.9"},
	}
	// Client Hints are a Chromium feature; sending them with a Firefox UA is an
	// inconsistency Cloudflare flags. Only emit them for Chrome/Chromium.
	if !c.firefox {
		req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

// GetBytes performs a GET and returns the body and status code.
func (c *Client) GetBytes(url string, extra map[string]string) ([]byte, int, error) {
	resp, err := c.Do(url, extra)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

// GetJSON fetches url and returns body bytes, erroring on non-2xx.
func (c *Client) GetJSON(url string, extra map[string]string) ([]byte, error) {
	b, status, err := c.GetBytes(url, extra)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", url, status)
	}
	return b, nil
}

// UserAgent returns the configured UA, useful for handing to ffmpeg.
func (c *Client) UserAgent() string { return c.userAgent }

// SetUserAgent updates the UA sent on subsequent requests, so a cf_clearance
// cookie can be paired with its matching browser UA without rebuilding.
func (c *Client) SetUserAgent(ua string) {
	if ua != "" {
		c.userAgent = ua
	}
}
