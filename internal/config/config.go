package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const (
	danbooruURL    = "https://danbooru.donmai.us"
	gelbooruURL    = "https://gelbooru.com"
	rule34URL      = "https://api.rule34.xxx"
	xbooruURL      = "https://xbooru.com"
	safebooruURL   = "https://safebooru.org"
	yandereURL     = "https://yande.re"
	konachanURL    = "https://konachan.com"
	sakugabooruURL = "https://sakugabooru.com"
)

type Family string

const (
	FamilyDanbooru Family = "danbooru"
	FamilyGelbooru Family = "gelbooru"
	FamilyMoebooru Family = "moebooru"
)

// URLEnv is the envar that may override DefaultURL, and is empty for sites whose address is always the same. Only
// konachan has one, because konachan.net is the SFW mirror of konachan.com.
type ClientSpec struct {
	Name          string
	Family        Family
	DefaultURL    string
	URLEnv        string
	RequiredCreds []string
	OptionalCreds []string
}

var clientSpecs = []ClientSpec{
	{
		Name: "danbooru", Family: FamilyDanbooru, DefaultURL: danbooruURL,
		OptionalCreds: []string{"DANBOORU_LOGIN", "DANBOORU_API_KEY"},
	},
	{
		Name: "gelbooru", Family: FamilyGelbooru, DefaultURL: gelbooruURL,
		RequiredCreds: []string{"GELBOORU_API_KEY", "GELBOORU_USER_ID"},
	},
	{
		Name: "rule34", Family: FamilyGelbooru, DefaultURL: rule34URL,
		RequiredCreds: []string{"RULE34_API_KEY", "RULE34_USER_ID"},
	},
	{
		Name: "xbooru", Family: FamilyGelbooru, DefaultURL: xbooruURL,
	},
	{
		Name: "safebooru", Family: FamilyGelbooru, DefaultURL: safebooruURL,
	},
	{
		Name: "yandere", Family: FamilyMoebooru, DefaultURL: yandereURL,
	},
	{
		Name: "konachan", Family: FamilyMoebooru, DefaultURL: konachanURL,
		URLEnv: "KONACHAN_URL",
	},
	{
		Name: "sakugabooru", Family: FamilyMoebooru, DefaultURL: sakugabooruURL,
	},
}

func ClientSpecs() []ClientSpec {
	out := make([]ClientSpec, len(clientSpecs))
	copy(out, clientSpecs)

	return out
}

func ClientSpecByName(name string) (ClientSpec, bool) {
	for _, spec := range clientSpecs {
		if spec.Name == name {
			return spec, true
		}
	}

	return ClientSpec{}, false
}

type Config struct {
	Host string `env:"HOST" envDefault:"0.0.0.0"`
	Port int    `env:"PORT" envDefault:"8080"`

	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	ErrorDetail string `env:"ERROR_DETAIL" envDefault:"useful"`

	APIKey string `env:"API_KEY"`

	Clients           string `env:"BOORU_CLIENTS"`
	DefaultClientsRaw string `env:"DEFAULT_CLIENTS" envDefault:"rule34,danbooru,gelbooru"`
	UserAgent         string `env:"USER_AGENT" envDefault:"booru-mcp/0.1.0"`
	DanbooruTierRaw   string `env:"DANBOORU_TIER" envDefault:"auto"`

	RateLimitRPS          float64 `env:"RATE_LIMIT_RPS" envDefault:"1"`
	RateLimitBurst        int     `env:"RATE_LIMIT_BURST" envDefault:"1"`
	RequestTimeoutSeconds int     `env:"REQUEST_TIMEOUT_SECONDS" envDefault:"20"`
	MaxLimit              int     `env:"MAX_LIMIT" envDefault:"100"`

	CacheTTLDays int    `env:"CACHE_TTL_DAYS" envDefault:"30"`
	DBPath       string `env:"DB_PATH" envDefault:"booru-mcp.db"`

	ContentRatingRaw string `env:"CONTENT_RATING" envDefault:"all"`
	BlockedTagsRaw   string `env:"BLOCKED_TAGS"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) VerboseErrors() bool {
	return strings.EqualFold(strings.TrimSpace(c.ErrorDetail), "verbose")
}

func (c Config) Env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func (c Config) EnabledClients() []string {
	raw := strings.TrimSpace(c.Clients)
	if raw == "" || strings.EqualFold(raw, "all") {
		return allClientNames()
	}

	return splitList(raw)
}

func (c Config) DefaultClients() []string {
	return splitList(c.DefaultClientsRaw)
}

func (c Config) ClientURL(name string) string {
	spec, ok := ClientSpecByName(name)
	if !ok {
		return ""
	}

	if spec.URLEnv != "" {
		if override := c.Env(spec.URLEnv); override != "" {
			return override
		}
	}

	return spec.DefaultURL
}

func (c Config) ClientActive(name string) (bool, string) {
	spec, ok := ClientSpecByName(name)
	if !ok {
		return false, fmt.Sprintf("unknown client %q", name)
	}

	missing := make([]string, 0, len(spec.RequiredCreds))
	for _, key := range spec.RequiredCreds {
		if c.Env(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return false, fmt.Sprintf("%s requires credentials that are not set: %s", name, strings.Join(missing, ", "))
	}

	return true, ""
}

func (c Config) BlockedTags() []string {
	seen := make(map[string]bool)
	out := make([]string, 0)

	for _, raw := range splitList(c.BlockedTagsRaw) {
		tag := booru.NormalizeTag(raw)
		if tag == "" || seen[tag] {
			continue
		}

		seen[tag] = true
		out = append(out, tag)
	}

	return out
}

// DanbooruTier is the normalized account tier that determines the per-search tag cap. `auto` means anonymous without
// credentials and gold with them, which matches what the credential hint has always promised.
func (c Config) DanbooruTier() string {
	tier := strings.ToLower(strings.TrimSpace(c.DanbooruTierRaw))
	if tier == "" {
		tier = "auto"
	}

	return tier
}

func (c Config) ResolvedDanbooruTier() string {
	tier := c.DanbooruTier()
	if tier != "auto" {
		return tier
	}

	if c.Env("DANBOORU_LOGIN") != "" && c.Env("DANBOORU_API_KEY") != "" {
		return "gold"
	}

	return "anonymous"
}

func (c Config) DanbooruTagLimit() int {
	switch c.ResolvedDanbooruTier() {
	case "gold":
		return 6
	case "platinum", "builder":
		return 0
	default:
		return 2
	}
}

func (c Config) CacheTTL() time.Duration {
	return time.Duration(c.CacheTTLDays) * 24 * time.Hour
}

func (c Config) ContentRating() (booru.Rating, error) {
	raw := strings.TrimSpace(c.ContentRatingRaw)
	rating, err := booru.ParseRating(raw)
	if err != nil {
		return "", fmt.Errorf("CONTENT_RATING: %q is not valid; accepted values are %s", raw, ratingValues())
	}

	return rating, nil
}

func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API_KEY is required")
	}

	if c.RateLimitRPS <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPS must be greater than 0, got %v", c.RateLimitRPS)
	}

	if c.RateLimitBurst < 1 {
		return fmt.Errorf("RATE_LIMIT_BURST must be at least 1, got %d", c.RateLimitBurst)
	}

	if c.CacheTTLDays < 0 {
		return fmt.Errorf("CACHE_TTL_DAYS must not be negative, got %d", c.CacheTTLDays)
	}

	if c.MaxLimit < 1 {
		return fmt.Errorf("MAX_LIMIT must be at least 1, got %d", c.MaxLimit)
	}

	if _, err := c.ContentRating(); err != nil {
		return err
	}

	if tier := c.DanbooruTier(); !validDanbooruTier(tier) {
		return fmt.Errorf("DANBOORU_TIER: %q is not valid; accepted values are %s", tier, danbooruTierValues())
	}

	enabled := c.EnabledClients()

	enabledSet := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		if _, ok := ClientSpecByName(name); !ok {
			return fmt.Errorf("BOORU_CLIENTS: unknown client %q", name)
		}

		enabledSet[name] = true
	}

	for _, name := range c.DefaultClients() {
		spec, ok := ClientSpecByName(name)
		if !ok {
			return fmt.Errorf("DEFAULT_CLIENTS: unknown client %q", name)
		}

		if !enabledSet[name] {
			return fmt.Errorf("DEFAULT_CLIENTS: %q is not enabled by BOORU_CLIENTS", spec.Name)
		}
	}

	for _, name := range enabled {
		spec, _ := ClientSpecByName(name)

		raw := c.ClientURL(name)

		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			label := spec.URLEnv
			if label == "" {
				label = spec.Name + " URL"
			}

			return fmt.Errorf("%s: %q is not an absolute http or https URL", label, raw)
		}
	}

	return nil
}

func allClientNames() []string {
	names := make([]string, 0, len(clientSpecs))

	for _, spec := range clientSpecs {
		names = append(names, spec.Name)
	}

	return names
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func validDanbooruTier(tier string) bool {
	for _, known := range danbooruTiers {
		if tier == known {
			return true
		}
	}

	return false
}

var danbooruTiers = []string{"auto", "anonymous", "member", "gold", "platinum", "builder"}

func danbooruTierValues() string {
	return strings.Join(danbooruTiers, ", ")
}

func ratingValues() string {
	values := booru.Ratings()
	names := make([]string, 0, len(values))

	for _, rating := range values {
		names = append(names, rating.String())
	}

	return strings.Join(names, ", ")
}
