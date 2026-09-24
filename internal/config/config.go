package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// maxTagLimit is the largest page the tag listing is asked for. Kept at 100 because the Danbooru adapter reads a window
// from pages of that size and a larger limit would span more than two pages.
const maxTagLimit = 100

type Config struct {
	Host string `env:"HOST" envDefault:"0.0.0.0"`
	Port int    `env:"PORT" envDefault:"8080"`

	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	ErrorDetail string `env:"ERROR_DETAIL" envDefault:"useful"`

	APIKey string `env:"API_KEY"`

	UserAgent string `env:"USER_AGENT" envDefault:"booru-mcp/0.1.0"`

	RateLimitRPS          float64 `env:"RATE_LIMIT_RPS" envDefault:"1"`
	RateLimitBurst        int     `env:"RATE_LIMIT_BURST" envDefault:"1"`
	RequestTimeoutSeconds int     `env:"REQUEST_TIMEOUT_SECONDS" envDefault:"20"`

	MaxLimit  int `env:"MAX_LIMIT" envDefault:"100"`
	MaxOffset int `env:"MAX_OFFSET" envDefault:"1000"`

	DanbooruLogin  string `env:"DANBOORU_LOGIN"`
	DanbooruAPIKey string `env:"DANBOORU_API_KEY"`

	BlockedTagsRaw string `env:"BLOCKED_TAGS"`

	// RelationIndexRefreshHours is how often the canonical alias and implication graphs are re-crawled. Zero disables
	// the index, leaving every lookup a miss; exact mode still answers from a single upstream call.
	RelationIndexRefreshHours int `env:"RELATION_INDEX_REFRESH_HOURS" envDefault:"24"`
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

func (c Config) DanbooruAuthenticated() bool {
	return c.DanbooruLogin != "" && c.DanbooruAPIKey != ""
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

func (c Config) Validate() error {
	if c.RateLimitRPS <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPS must be greater than 0, got %v", c.RateLimitRPS)
	}

	if c.RateLimitBurst < 1 {
		return fmt.Errorf("RATE_LIMIT_BURST must be at least 1, got %d", c.RateLimitBurst)
	}

	if c.MaxLimit < 1 || c.MaxLimit > maxTagLimit {
		return fmt.Errorf("MAX_LIMIT must be between 1 and %d, got %d", maxTagLimit, c.MaxLimit)
	}

	if c.MaxOffset < 1 {
		return fmt.Errorf("MAX_OFFSET must be at least 1, got %d", c.MaxOffset)
	}

	if c.RelationIndexRefreshHours < 0 {
		return fmt.Errorf("RELATION_INDEX_REFRESH_HOURS must be zero or greater, got %d", c.RelationIndexRefreshHours)
	}

	return nil
}

func (c Config) RelationIndexInterval() time.Duration {
	return time.Duration(c.RelationIndexRefreshHours) * time.Hour
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
