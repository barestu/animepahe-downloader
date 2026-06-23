// Package config loads runtime configuration: the (rotating) AnimePahe base
// URLs, output directory and HTTP identity. Precedence, highest first:
//
//	--base-url flag  >  ANIMEPAHE_BASE_URL env  >  ~/.config/animepahe-dl/config.json  >  built-in defaults
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultUserAgent matches the Chrome TLS profile used by the client so the
// JA3 fingerprint and UA string agree (Cloudflare cross-checks them).
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// DefaultBaseURLs are the domains known to work at time of writing. The site
// rotates domains, so this is only a starting point — override via flag/env/file.
var DefaultBaseURLs = []string{
	"https://animepahe.pw",
	"https://animepahe.com",
	"https://animepahe.org",
}

// Config is the resolved runtime configuration.
type Config struct {
	BaseURLs    []string `json:"base_urls"`
	OutputDir   string   `json:"output_dir"`
	UserAgent   string   `json:"user_agent"`
	Concurrency int      `json:"concurrency"`
	// Cookie is a raw Cookie header (e.g. "cf_clearance=...") harvested from a
	// browser to pass Cloudflare's managed challenge. Must be paired with the
	// same browser's UserAgent.
	Cookie string `json:"cookie"`
}

// Load resolves configuration applying the documented precedence. flagBaseURL
// is the value of --base-url (empty if unset); it wins over everything else.
func Load(flagBaseURL string) Config {
	cfg := Config{
		BaseURLs:    append([]string(nil), DefaultBaseURLs...),
		OutputDir:   ".",
		UserAgent:   DefaultUserAgent,
		Concurrency: 4,
	}

	// File overrides defaults.
	fileCfg := loadFile()
	if len(fileCfg.BaseURLs) > 0 {
		cfg.BaseURLs = fileCfg.BaseURLs
	}
	if fileCfg.OutputDir != "" {
		cfg.OutputDir = fileCfg.OutputDir
	}
	if fileCfg.UserAgent != "" {
		cfg.UserAgent = fileCfg.UserAgent
	}
	if fileCfg.Concurrency > 0 {
		cfg.Concurrency = fileCfg.Concurrency
	}
	if fileCfg.Cookie != "" {
		cfg.Cookie = fileCfg.Cookie
	}

	// Env overrides file. Comma-separated list allowed.
	if env := os.Getenv("ANIMEPAHE_BASE_URL"); env != "" {
		cfg.BaseURLs = splitURLs(env)
	}
	if env := os.Getenv("ANIMEPAHE_COOKIE"); env != "" {
		cfg.Cookie = env
	}
	if env := os.Getenv("ANIMEPAHE_USER_AGENT"); env != "" {
		cfg.UserAgent = env
	}

	// Flag overrides env.
	if flagBaseURL != "" {
		cfg.BaseURLs = splitURLs(flagBaseURL)
	}

	// Normalise: strip trailing slashes.
	for i, u := range cfg.BaseURLs {
		cfg.BaseURLs[i] = strings.TrimRight(strings.TrimSpace(u), "/")
	}
	return cfg
}

// loadFile reads only the JSON config file, with no defaults/env/flag merging.
// A missing or unreadable file yields a zero-value Config (not an error), so
// callers that mutate-and-Save don't bake built-in defaults into the file.
func loadFile() Config {
	var cfg Config
	path := ConfigPath()
	if path == "" {
		return cfg
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

// Set persists a single config key to the JSON file, preserving all other
// existing keys (including an auto-saved cf_clearance cookie). It reads only
// the file (loadFile), so it never writes built-in defaults or env values.
// Supported keys: "output-dir", "base-url" (comma-separated list), "user-agent".
func Set(key, value string) error {
	cfg := loadFile()
	switch key {
	case "output-dir":
		cfg.OutputDir = value
	case "base-url":
		cfg.BaseURLs = splitURLs(value)
		for i, u := range cfg.BaseURLs {
			cfg.BaseURLs[i] = strings.TrimRight(u, "/")
		}
	case "user-agent":
		cfg.UserAgent = value
	default:
		return fmt.Errorf("unknown config key %q (valid: output-dir, base-url, user-agent)", key)
	}
	return Save(cfg)
}

// SaveCredentials persists a freshly-entered cf_clearance cookie + matching UA,
// preserving other existing file keys. Routes through loadFile so it never
// bakes built-in defaults or env-derived values permanently into the file.
func SaveCredentials(cookie, userAgent string) error {
	cfg := loadFile()
	cfg.Cookie = cookie
	if userAgent != "" {
		cfg.UserAgent = userAgent
	}
	return Save(cfg)
}

// Save writes cfg to the JSON config file, creating the directory if needed.
// Used to persist a freshly-entered cf_clearance cookie + UA so later runs
// reuse them without prompting.
func Save(cfg Config) error {
	path := ConfigPath()
	if path == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ConfigPath returns the path to the JSON config file (may not exist).
func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "animepahe-dl", "config.json")
}

// HasFFmpeg reports whether ffmpeg is available on PATH (required only for the
// HLS download path).
func HasFFmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func splitURLs(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
