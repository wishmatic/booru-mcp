package config

import (
	"strings"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		Host:                  "127.0.0.1",
		Port:                  8080,
		APIKey:                "key",
		UserAgent:             "booru-mcp/0.1.0",
		RateLimitRPS:          1,
		RateLimitBurst:        1,
		RequestTimeoutSeconds: 20,
		MaxLimit:              100,
		MaxOffset:             1000,
	}
}

func TestAddrAndVerboseErrors(t *testing.T) {
	cfg := testConfig()
	cfg.Port = 9000

	if got := cfg.Addr(); got != "127.0.0.1:9000" {
		t.Errorf("Addr() = %q", got)
	}

	if cfg.VerboseErrors() {
		t.Error("VerboseErrors() = true, want false by default")
	}

	cfg.ErrorDetail = "verbose"

	if !cfg.VerboseErrors() {
		t.Error("VerboseErrors() = false, want true")
	}
}

func TestDanbooruAuthenticated(t *testing.T) {
	tests := map[string]struct {
		login, key string
		want       bool
	}{
		"both set":   {login: "me", key: "secret", want: true},
		"login only": {login: "me"},
		"key only":   {key: "secret"},
		"neither":    {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig()
			cfg.DanbooruLogin = tt.login
			cfg.DanbooruAPIKey = tt.key

			if got := cfg.DanbooruAuthenticated(); got != tt.want {
				t.Errorf("DanbooruAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockedTags(t *testing.T) {
	cfg := testConfig()
	cfg.BlockedTagsRaw = " Some Tag , other ,, some_tag "

	got := cfg.BlockedTags()
	if len(got) != 2 || got[0] != "some_tag" || got[1] != "other" {
		t.Fatalf("BlockedTags() = %v, want [some_tag other]", got)
	}

	cfg.BlockedTagsRaw = ""

	if got := cfg.BlockedTags(); len(got) != 0 {
		t.Fatalf("BlockedTags() = %v, want empty", got)
	}
}

func TestValidateAcceptsDefaults(t *testing.T) {
	if err := testConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want the defaults to pass", err)
	}
}

func TestValidateRejectsOutOfRangeValues(t *testing.T) {
	tests := map[string]struct {
		apply     func(*Config)
		wantNamed string
	}{
		"zero rps":          {apply: func(c *Config) { c.RateLimitRPS = 0 }, wantNamed: "RATE_LIMIT_RPS"},
		"negative rps":      {apply: func(c *Config) { c.RateLimitRPS = -1 }, wantNamed: "RATE_LIMIT_RPS"},
		"zero burst":        {apply: func(c *Config) { c.RateLimitBurst = 0 }, wantNamed: "RATE_LIMIT_BURST"},
		"zero max limit":    {apply: func(c *Config) { c.MaxLimit = 0 }, wantNamed: "MAX_LIMIT"},
		"max limit too big": {apply: func(c *Config) { c.MaxLimit = 101 }, wantNamed: "MAX_LIMIT"},
		"zero max offset":   {apply: func(c *Config) { c.MaxOffset = 0 }, wantNamed: "MAX_OFFSET"},
		"negative refresh":  {apply: func(c *Config) { c.ImplicationIndexRefreshHours = -1 }, wantNamed: "IMPLICATION_INDEX_REFRESH_HOURS"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig()
			tt.apply(&cfg)

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantNamed) {
				t.Fatalf("Validate() error = %v, want it to name %s", err, tt.wantNamed)
			}
		})
	}
}

func TestImplicationIndexInterval(t *testing.T) {
	tests := map[int]time.Duration{
		0:  0,
		1:  time.Hour,
		24: 24 * time.Hour,
	}

	for hours, want := range tests {
		cfg := testConfig()
		cfg.ImplicationIndexRefreshHours = hours

		if got := cfg.ImplicationIndexInterval(); got != want {
			t.Errorf("ImplicationIndexInterval() for %d = %v, want %v", hours, got, want)
		}
	}
}

func TestLoadValues(t *testing.T) {
	t.Setenv("API_KEY", "secret")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9100")
	t.Setenv("RATE_LIMIT_RPS", "2.5")
	t.Setenv("MAX_LIMIT", "42")
	t.Setenv("MAX_OFFSET", "7")
	t.Setenv("DANBOORU_LOGIN", "someone")
	t.Setenv("DANBOORU_API_KEY", "danbooru-key")
	t.Setenv("BLOCKED_TAGS", "bad_tag")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIKey != "secret" || cfg.Port != 9100 || cfg.RateLimitRPS != 2.5 {
		t.Fatalf("Load() = %+v, want the configured values", cfg)
	}

	if cfg.MaxLimit != 42 || cfg.MaxOffset != 7 {
		t.Fatalf("Load() bounds = %d/%d, want 42/7", cfg.MaxLimit, cfg.MaxOffset)
	}

	if cfg.ImplicationIndexInterval() != 24*time.Hour {
		t.Errorf("ImplicationIndexInterval() = %v, want the 24 hour default", cfg.ImplicationIndexInterval())
	}

	if !cfg.DanbooruAuthenticated() {
		t.Error("DanbooruAuthenticated() = false, want the configured credentials")
	}

	if got := cfg.BlockedTags(); len(got) != 1 || got[0] != "bad_tag" {
		t.Errorf("BlockedTags() = %v, want [bad_tag]", got)
	}
}
