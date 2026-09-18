package catalog

import (
	"context"
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/store"
)

type TagsInput struct {
	Query    string
	Clients  []string
	Category booru.TagCategory
	Limit    int
	Refresh  bool
}

type TagsResult struct {
	Tags     []booru.FusedTag
	Skipped  []booru.Skipped
	Warnings []string
}

func (s *Service) Tags(ctx context.Context, input TagsInput) (TagsResult, error) {
	resolution, err := s.resolve(input.Clients)
	if err != nil {
		return TagsResult{}, err
	}

	limit := s.clampLimit(input.Limit, defaultTagLimit)
	query := booru.NormalizeTag(input.Query)

	var (
		sets      []tagSet
		warnings  []string
		lastErr   error
		successes int
	)

	for _, entry := range resolution.active {
		tags, err := s.clientTags(ctx, entry, query, input.Category, s.fetchLimit(limit), input.Refresh)
		if err != nil {
			lastErr = err
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name, err))

			continue
		}

		successes++
		sets = append(sets, tagSet{Client: entry.Name, Tags: tags})
	}

	result := TagsResult{Skipped: resolution.skipped, Warnings: warnings}

	if len(resolution.active) > 0 && successes == 0 && lastErr != nil {
		return result, fmt.Errorf("catalog: all clients failed: %w", lastErr)
	}

	result.Tags = capFused(s.filterTagNames(mergeTagSets(sets)), limit)

	return result, nil
}

func (s *Service) clientTags(
	ctx context.Context,
	entry booru.Entry,
	query string,
	category booru.TagCategory,
	limit int,
	refresh bool,
) ([]booru.Tag, error) {
	if s.opts.CacheTTL <= 0 {
		return entry.Provider.SearchTags(ctx, booru.TagQuery{Query: query, Category: category, Limit: limit})
	}

	cached, err := s.store.SearchTags(ctx, store.TagFilter{Client: entry.Name, Query: query, Category: category, Limit: limit})
	if err != nil {
		return nil, err
	}

	if len(cached) > 0 && !refresh && !s.hasStalePopular(cached) {
		return cachedTags(cached), nil
	}

	tags, fetchErr := entry.Provider.SearchTags(ctx, booru.TagQuery{Query: query, Category: category, Limit: limit})
	if fetchErr != nil {
		if len(cached) > 0 {
			return cachedTags(cached), nil
		}

		return nil, fetchErr
	}

	if err := s.store.UpsertTags(ctx, entry.Name, tags, s.now()); err != nil {
		return nil, err
	}

	return tags, nil
}

func (s *Service) hasStalePopular(cached []store.CachedTag) bool {
	for _, tag := range cached {
		if tag.IsPopular && s.isStale(tag.FetchedAt) {
			return true
		}
	}

	return false
}

func cachedTags(cached []store.CachedTag) []booru.Tag {
	tags := make([]booru.Tag, 0, len(cached))

	for _, tag := range cached {
		tags = append(tags, tag.Tag)
	}

	return tags
}

func capFused(tags []booru.FusedTag, limit int) []booru.FusedTag {
	if limit > 0 && len(tags) > limit {
		return tags[:limit]
	}

	return tags
}
