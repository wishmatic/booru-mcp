package catalog

import (
	"context"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const (
	DefaultTagLimit = 25

	defaultMaxLimit  = 100
	defaultMaxOffset = 1000
)

// TagSource is the capabilities the service needs from the booru client: the paged substring search plus the three
// canonical lookups that back exact mode. It is declared here, where it is consumed, so the policy stays testable
// without an HTTP server.
type TagSource interface {
	SearchTags(ctx context.Context, query booru.TagQuery) (booru.TagPage, error)
	FindTag(ctx context.Context, name string) (booru.Tag, bool, error)
	AliasTarget(ctx context.Context, name string) (string, bool, error)
	Implications(ctx context.Context, name string) ([]string, error)
}

// RelationIndex answers the canonical alias and implication graphs from the ingested index without touching the
// network. A nil index is valid and simply means nothing is known locally. Ready distinguishes "no such alias" from
// "not crawled yet", so callers can decide whether to fall back to an upstream lookup.
type RelationIndex interface {
	Implications(name string) []string
	Synonyms(name string) []string
	Canonical(name string) (string, bool)
	Ready() bool
}

type Options struct {
	BlockedTags []string
	MaxLimit    int
	MaxOffset   int
	Relations   RelationIndex
	Clock       func() time.Time
}

// Service owns the policy around a tag search: what a valid search is, which window sizes are allowed, which tag
// names an operator has blocked, and how an alias, an implication, and an empty result are reported.
type Service struct {
	source  TagSource
	opts    Options
	blocked map[string]bool
	now     func() time.Time
}

func New(source TagSource, opts Options) *Service {
	if opts.MaxLimit < 1 {
		opts.MaxLimit = defaultMaxLimit
	}

	if opts.MaxOffset < 1 {
		opts.MaxOffset = defaultMaxOffset
	}

	if opts.Clock == nil {
		opts.Clock = time.Now
	}

	return &Service{source: source, opts: opts, blocked: blockedSet(opts.BlockedTags), now: opts.Clock}
}

func blockedSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))

	for _, name := range names {
		set[name] = true
	}

	return set
}

func (s *Service) filterBlocked(tags []booru.Tag) []booru.Tag {
	if len(s.blocked) == 0 {
		return tags
	}

	kept := make([]booru.Tag, 0, len(tags))

	for _, tag := range tags {
		if s.blocked[tag.Name] {
			continue
		}

		kept = append(kept, tag)
	}

	return kept
}

func (s *Service) snapshotDate() string {
	return s.now().UTC().Format("2006-01-02")
}
