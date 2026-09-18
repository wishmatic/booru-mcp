package config

import (
	"os"
	"strings"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	prev, had := os.LookupEnv(key)

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}

	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)

			return
		}

		_ = os.Unsetenv(key)
	})
}

func testConfig(t *testing.T) Config {
	t.Helper()

	for _, key := range []string{"BOORU_CLIENTS", "KONACHAN_URL"} {
		unsetEnv(t, key)
	}

	return Config{
		APIKey:                "key",
		DefaultClientsRaw:     "rule34,danbooru,gelbooru",
		UserAgent:             "booru-mcp/0.1.0",
		RateLimitRPS:          1,
		RateLimitBurst:        1,
		RequestTimeoutSeconds: 20,
		MaxLimit:              100,
		CacheTTLDays:          30,
		DBPath:                "booru-mcp.db",
		ContentRatingRaw:      "all",
	}
}

func TestEnabledClientsDefault(t *testing.T) {
	cfg := testConfig(t)

	enabled := cfg.EnabledClients()

	if len(enabled) != len(clientSpecs) {
		t.Fatalf("EnabledClients() = %d clients, want the full roster of %d", len(enabled), len(clientSpecs))
	}

	enabledSet := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		enabledSet[name] = true
	}

	for _, name := range []string{"danbooru", "gelbooru", "rule34", "xbooru", "safebooru", "yandere", "konachan", "sakugabooru"} {
		if !enabledSet[name] {
			t.Errorf("EnabledClients() is missing %q", name)
		}
	}
}

func TestEnabledClientsAll(t *testing.T) {
	cfg := testConfig(t)
	cfg.Clients = "all"

	if got := len(cfg.EnabledClients()); got != len(clientSpecs) {
		t.Fatalf("EnabledClients() = %d clients, want %d", got, len(clientSpecs))
	}
}

func TestEnabledClientsExplicit(t *testing.T) {
	cfg := testConfig(t)
	cfg.Clients = "danbooru, rule34"

	if got := cfg.EnabledClients(); len(got) != 2 || got[0] != "danbooru" || got[1] != "rule34" {
		t.Fatalf("EnabledClients() = %v, want [danbooru rule34]", got)
	}
}

func TestDefaultClients(t *testing.T) {
	cfg := testConfig(t)

	if got := cfg.DefaultClients(); len(got) != 3 || got[0] != "rule34" || got[1] != "danbooru" || got[2] != "gelbooru" {
		t.Fatalf("DefaultClients() = %v, want [rule34 danbooru gelbooru]", got)
	}
}

func TestRemovedClientsAreUnknown(t *testing.T) {
	removed := []string{"aibooru", "realbooru", "tbib", "e621", "e926", "derpibooru", "twibooru", "furbooru"}

	for _, name := range removed {
		if _, ok := ClientSpecByName(name); ok {
			t.Errorf("ClientSpecByName(%q) found a client that should be removed", name)
		}
	}
}

func TestClientURLOverride(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("KONACHAN_URL", "https://konachan.net")

	if got := cfg.ClientURL("konachan"); got != "https://konachan.net" {
		t.Errorf("ClientURL(konachan) = %q, want the override", got)
	}

	if got := cfg.ClientURL("danbooru"); got != "https://danbooru.donmai.us" {
		t.Errorf("ClientURL(danbooru) = %q, want the default", got)
	}
}

func TestStaticURLsIgnoreEnvOverrides(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("DANBOORU_URL", "not-a-url")

	if got := cfg.ClientURL("danbooru"); got != "https://danbooru.donmai.us" {
		t.Errorf("ClientURL(danbooru) = %q, want the constant to win", got)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want a fixed client URL to ignore the envar", err)
	}
}

func TestCacheTTL(t *testing.T) {
	cfg := testConfig(t)

	if got := cfg.CacheTTL(); got.Hours() != 720 {
		t.Errorf("CacheTTL() = %v, want 720h", got)
	}

	cfg.CacheTTLDays = 0

	if got := cfg.CacheTTL(); got != 0 {
		t.Errorf("CacheTTL() = %v, want 0", got)
	}
}

func TestBlockedTags(t *testing.T) {
	cfg := testConfig(t)
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

func TestValidateRateLimit(t *testing.T) {
	for _, rps := range []float64{0, -1} {
		cfg := testConfig(t)
		cfg.RateLimitRPS = rps

		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "RATE_LIMIT_RPS") {
			t.Fatalf("Validate() error = %v, want it to name RATE_LIMIT_RPS", err)
		}
	}
}

func TestValidateContentRating(t *testing.T) {
	cfg := testConfig(t)
	cfg.ContentRatingRaw = "nsfw"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "CONTENT_RATING") {
		t.Fatalf("Validate() error = %v, want it to name CONTENT_RATING", err)
	}

	for _, value := range []string{"general", "sensitive", "questionable", "explicit", "all"} {
		if !strings.Contains(err.Error(), value) {
			t.Errorf("error %q does not list the accepted value %q", err, value)
		}
	}
}

func TestContentRatingDefault(t *testing.T) {
	cfg := testConfig(t)

	if got, err := cfg.ContentRating(); err != nil || got != "all" {
		t.Fatalf("ContentRating() = %q, %v, want all", got, err)
	}
}

func TestValidateUnknownClient(t *testing.T) {
	cfg := testConfig(t)
	cfg.Clients = "danbooru,nope"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("Validate() error = %v, want it to name nope", err)
	}
}

func TestValidateDefaultClientNotEnabled(t *testing.T) {
	cfg := testConfig(t)
	cfg.Clients = "danbooru"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DEFAULT_CLIENTS") {
		t.Fatalf("Validate() error = %v, want it to name DEFAULT_CLIENTS", err)
	}
}

func TestValidateBadURLOverride(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("KONACHAN_URL", "not-a-url")

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "KONACHAN_URL") {
		t.Fatalf("Validate() error = %v, want it to name KONACHAN_URL", err)
	}
}

func TestClientActiveMissingCredentials(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("RULE34_API_KEY", "")
	t.Setenv("RULE34_USER_ID", "")

	active, reason := cfg.ClientActive("rule34")
	if active {
		t.Fatal("ClientActive(rule34) = true, want false without credentials")
	}

	for _, want := range []string{"RULE34_API_KEY", "RULE34_USER_ID"} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason %q does not name %s", reason, want)
		}
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want missing credentials not to fail startup", err)
	}
}

func TestXbooruNeedsNoCredentials(t *testing.T) {
	cfg := testConfig(t)
	unsetEnv(t, "XBOORU_API_KEY")
	unsetEnv(t, "XBOORU_USER_ID")

	active, reason := cfg.ClientActive("xbooru")
	if !active {
		t.Fatalf("ClientActive(xbooru) = false (%s), want true without credentials", reason)
	}
}

func TestClientActiveReasonHidesSecrets(t *testing.T) {
	cfg := testConfig(t)
	t.Setenv("RULE34_API_KEY", "super-secret-value")
	t.Setenv("RULE34_USER_ID", "")

	active, reason := cfg.ClientActive("rule34")
	if active {
		t.Fatal("ClientActive(rule34) = true, want false")
	}

	if strings.Contains(reason, "super-secret-value") {
		t.Fatalf("reason %q leaks a credential", reason)
	}
}

func TestAddrAndVerboseErrors(t *testing.T) {
	cfg := testConfig(t)
	cfg.Host = "127.0.0.1"
	cfg.Port = 9000
	cfg.ErrorDetail = "verbose"

	if got := cfg.Addr(); got != "127.0.0.1:9000" {
		t.Errorf("Addr() = %q", got)
	}

	if !cfg.VerboseErrors() {
		t.Error("VerboseErrors() = false, want true")
	}
}

func TestLoadValues(t *testing.T) {
	t.Setenv("API_KEY", "secret")
	t.Setenv("RATE_LIMIT_RPS", "2.5")
	t.Setenv("CACHE_TTL_DAYS", "7")
	t.Setenv("CONTENT_RATING", "questionable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIKey != "secret" || cfg.RateLimitRPS != 2.5 || cfg.CacheTTLDays != 7 || cfg.ContentRatingRaw != "questionable" {
		t.Fatalf("Load() = %+v, want the configured values", cfg)
	}
}
