package catalog

import (
	"context"
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const noRelatedAPIReason = "the client has no related-tag API"

type RelatedInput struct {
	Tag     string
	Clients []string
	Limit   int
	Refresh bool
}

type RelatedResult struct {
	Tags     []booru.RelatedTag
	Skipped  []booru.Skipped
	Warnings []string
	Clients  []booru.ClientStatus
}

func (s *Service) Related(ctx context.Context, input RelatedInput) (RelatedResult, error) {
	resolution, err := s.resolve(input.Clients)
	if err != nil {
		return RelatedResult{}, err
	}

	tag := booru.NormalizeTag(input.Tag)
	limit := s.clampLimit(input.Limit, defaultTagLimit)

	var (
		tags      []booru.RelatedTag
		skipped   = resolution.skipped
		warnings  []string
		statuses  = skippedStatuses(resolution.skipped)
		lastErr   error
		queried   int
		successes int
	)

	for _, entry := range resolution.active {
		provider, ok := entry.Provider.(booru.RelatedTagProvider)
		if !ok {
			skipped = append(skipped, booru.Skipped{Client: entry.Name, Reason: noRelatedAPIReason})
			statuses = append(statuses, skippedStatus(entry.Name, noRelatedAPIReason))

			continue
		}

		queried++

		clientTags, err := s.clientRelated(ctx, entry.Name, provider, tag, limit, input.Refresh)
		if err != nil {
			lastErr = err
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name, err))
			statuses = append(statuses, errorStatus(entry.Name, err))

			continue
		}

		successes++
		tags = append(tags, clientTags...)
		statuses = append(statuses, okStatus(entry.Name, len(clientTags), ""))
	}

	result := RelatedResult{Skipped: skipped, Warnings: warnings, Clients: statuses}

	if queried > 0 && successes == 0 && lastErr != nil {
		return result, fmt.Errorf("catalog: all clients failed: %w", lastErr)
	}

	if limit > 0 && len(tags) > limit {
		tags = tags[:limit]
	}

	result.Tags = tags

	return result, nil
}

func (s *Service) clientRelated(
	ctx context.Context,
	client string,
	provider booru.RelatedTagProvider,
	tag string,
	limit int,
	refresh bool,
) ([]booru.RelatedTag, error) {
	fetch := func() ([]booru.RelatedTag, error) {
		return provider.RelatedTags(ctx, booru.RelatedQuery{Tag: tag, Limit: limit})
	}

	if s.opts.CacheTTL <= 0 {
		return fetch()
	}

	fetchedAt, cached, err := s.store.RelatedFetchedAt(ctx, client, tag)
	if err != nil {
		return nil, err
	}

	if cached && !refresh && !s.isStale(fetchedAt) {
		return s.store.Related(ctx, client, tag)
	}

	tags, err := fetch()
	if err != nil {
		if cached {
			return s.store.Related(ctx, client, tag)
		}

		return nil, err
	}

	if err := s.store.ReplaceRelated(ctx, client, tag, tags, s.now()); err != nil {
		return nil, err
	}

	return tags, nil
}
