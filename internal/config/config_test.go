package config

import (
	"encoding/json"
	"os"
	"testing"
)

// withTempConfig points os.UserConfigDir at a temp dir for the duration of the
// test, so loadFile/Set/Save operate on an isolated config.json. On macOS
// UserConfigDir derives from $HOME; on others from $XDG_CONFIG_HOME. Set both.
func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestLoadFileAbsent(t *testing.T) {
	withTempConfig(t)
	cfg := loadFile()
	if len(cfg.BaseURLs) != 0 || cfg.OutputDir != "" || cfg.UserAgent != "" || cfg.Cookie != "" || cfg.Concurrency != 0 {
		t.Fatalf("expected zero-value Config when file absent, got %+v", cfg)
	}
}

func TestLoadFilePresent(t *testing.T) {
	withTempConfig(t)
	want := Config{
		BaseURLs:  []string{"https://example.test"},
		OutputDir: "/tmp/out",
		UserAgent: "UA",
		Cookie:    "cf_clearance=abc",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := loadFile()
	if got.OutputDir != want.OutputDir || got.UserAgent != want.UserAgent || got.Cookie != want.Cookie {
		t.Fatalf("loadFile mismatch: got %+v want %+v", got, want)
	}
	if len(got.BaseURLs) != 1 || got.BaseURLs[0] != want.BaseURLs[0] {
		t.Fatalf("BaseURLs mismatch: got %v", got.BaseURLs)
	}
}

func TestSetRoundTripPreservesOtherKeys(t *testing.T) {
	withTempConfig(t)
	// Seed an auto-saved cookie + UA, as the Cloudflare flow would.
	if err := SaveCredentials("cf_clearance=secret", "FirefoxUA"); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	// Persist output-dir; must not touch the cookie or UA.
	if err := Set("output-dir", "/tmp/dl"); err != nil {
		t.Fatalf("Set output-dir: %v", err)
	}
	got := loadFile()
	if got.OutputDir != "/tmp/dl" {
		t.Fatalf("output-dir not persisted: %+v", got)
	}
	if got.Cookie != "cf_clearance=secret" {
		t.Fatalf("Set clobbered cookie: %q", got.Cookie)
	}
	if got.UserAgent != "FirefoxUA" {
		t.Fatalf("Set clobbered user-agent: %q", got.UserAgent)
	}

	// base-url splits + trims trailing slashes.
	if err := Set("base-url", "https://a.test/, https://b.test"); err != nil {
		t.Fatalf("Set base-url: %v", err)
	}
	got = loadFile()
	if len(got.BaseURLs) != 2 || got.BaseURLs[0] != "https://a.test" || got.BaseURLs[1] != "https://b.test" {
		t.Fatalf("base-url not parsed: %v", got.BaseURLs)
	}
	if got.Cookie != "cf_clearance=secret" {
		t.Fatalf("Set base-url clobbered cookie: %q", got.Cookie)
	}
}

func TestSetUnknownKey(t *testing.T) {
	withTempConfig(t)
	if err := Set("bogus", "x"); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSetDoesNotBakeDefaults(t *testing.T) {
	withTempConfig(t)
	if err := Set("output-dir", "/tmp/dl"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Read raw JSON: must contain only the set key, not built-in defaults.
	b, err := os.ReadFile(ConfigPath())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var stored Config
	_ = json.Unmarshal(b, &stored)
	if stored.UserAgent == DefaultUserAgent {
		t.Fatal("Set baked DefaultUserAgent into the file")
	}
	if len(stored.BaseURLs) != 0 {
		t.Fatalf("Set baked default base URLs into the file: %v", stored.BaseURLs)
	}
}
