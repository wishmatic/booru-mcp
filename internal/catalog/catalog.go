package catalog

import (
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/store"
)

const (
	defaultSearchLimit  = 20
	defaultTagLimit     = 25
	popularFanOutFactor = 4
)

type Options struct {
	DefaultClients []string
	CacheTTL       time.Duration
	ContentRating  booru.Rating
	BlockedTags    []string
	MaxLimit       int
}

type Service struct {
	registry *booru.Registry
	store    *store.Client
	opts     Options
	now      func() time.Time
}

func New(registry *booru.Registry, storeClient *store.Client, opts Options) *Service {
	if opts.MaxLimit < 1 {
		opts.MaxLimit = 100
	}

	return &Service{registry: registry, store: storeClient, opts: opts, now: time.Now}
}

func (s *Service) clampLimit(requested, fallback int) int {
	if requested <= 0 {
		requested = fallback
	}

	if requested > s.opts.MaxLimit {
		requested = s.opts.MaxLimit
	}

	return requested
}

func (s *Service) fetchLimit(limit int) int {
	fanned := limit * popularFanOutFactor
	if fanned < limit {
		fanned = limit
	}

	if fanned > s.opts.MaxLimit {
		fanned = s.opts.MaxLimit
	}

	return fanned
}

func (s *Service) isStale(fetchedAt time.Time) bool {
	ttl := s.opts.CacheTTL
	if ttl <= 0 {
		return true
	}

	return s.now().Sub(fetchedAt) >= ttl
}

func blockedSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))

	for _, name := range names {
		set[name] = true
	}

	return set
}
