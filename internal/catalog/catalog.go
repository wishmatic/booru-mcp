package catalog

import (
	"context"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const (
	DefaultTagLimit = 25

	defaultMaxLimit  = 100
	defaultMaxOffset = 1000
)

// TagSource is the one capability the service needs from the booru client. It is declared here, where it is consumed,
// so the search and its policy stay testable without an HTTP server.
type TagSource interface {
	SearchTags(ctx context.Context, query booru.TagQuery) (booru.TagPage, error)
}

type Options struct {
	BlockedTags []string
	MaxLimit    int
	MaxOffset   int
}

// Service owns the policy around a tag search: what a valid search is, which window sizes are allowed, and which tag
// names an operator has blocked.
type Service struct {
	source  TagSource
	opts    Options
	blocked map[string]bool
}

func New(source TagSource, opts Options) *Service {
	if opts.MaxLimit < 1 {
		opts.MaxLimit = defaultMaxLimit
	}

	if opts.MaxOffset < 1 {
		opts.MaxOffset = defaultMaxOffset
	}

	return &Service{source: source, opts: opts, blocked: blockedSet(opts.BlockedTags)}
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
