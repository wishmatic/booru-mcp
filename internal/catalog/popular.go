package catalog

import (
	"context"
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/store"
)

type PopularInput struct {
	Clients  []string
	Category booru.TagCategory
	Limit    int
	Refresh  bool
}

type PopularResult struct {
	Tags     []booru.FusedTag
	Skipped  []booru.Skipped
	Warnings []string
}

func (s *Service) Popular(ctx context.Context, input PopularInput) (PopularResult, error) {
	resolution, err := s.resolve(input.Clients)
	if err != nil {
		return PopularResult{}, err
	}

	limit := s.clampLimit(input.Limit, defaultTagLimit)
	perClient := make(map[string][]booru.Tag, len(resolution.active))

	var (
		warnings  []string
		lastErr   error
		successes int
	)

	for _, entry := range resolution.active {
		tags, err := s.clientPopular(ctx, entry, input.Category, s.fetchLimit(limit), input.Refresh)
		if err != nil {
			lastErr = err
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name, err))

			continue
		}

		successes++
		perClient[entry.Name] = tags
	}

	result := PopularResult{Skipped: resolution.skipped, Warnings: warnings}

	if len(resolution.active) > 0 && successes == 0 && lastErr != nil {
		return result, fmt.Errorf("catalog: all clients failed: %w", lastErr)
	}

	result.Tags = capFused(s.filterTagNames(Fuse(perClient)), limit)

	return result, nil
}

func (s *Service) clientPopular(
	ctx context.Context,
	entry booru.Entry,
	category booru.TagCategory,
	limit int,
	refresh bool,
) ([]booru.Tag, error) {
	fetch := func() ([]booru.Tag, error) {
		return entry.Provider.PopularTags(ctx, booru.PopularQuery{Category: category, Limit: limit})
	}

	if s.opts.CacheTTL <= 0 {
		return fetch()
	}

	cached, err := s.store.Popular(ctx, entry.Name)
	if err != nil {
		return nil, err
	}

	if len(cached) > 0 && !refresh && !s.anyStale(cached) {
		return cachedTags(cached), nil
	}

	tags, err := fetch()
	if err != nil {
		if len(cached) > 0 {
			return cachedTags(cached), nil
		}

		return nil, err
	}

	at := s.now()

	if err := s.store.ReplacePopular(ctx, entry.Name, tags, at); err != nil {
		return nil, err
	}

	if err := s.store.UpsertTags(ctx, entry.Name, tags, at); err != nil {
		return nil, err
	}

	return tags, nil
}

func (s *Service) anyStale(cached []store.CachedTag) bool {
	for _, tag := range cached {
		if s.isStale(tag.FetchedAt) {
			return true
		}
	}

	return false
}
