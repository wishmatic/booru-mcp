package catalog

import (
	"context"
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type SearchInput struct {
	Tags    string
	Clients []string
	Limit   int
	Page    int
	Rating  booru.Rating
	Random  bool
}

type SearchResult struct {
	Posts    []booru.Post
	Skipped  []booru.Skipped
	Warnings []string
}

func (s *Service) Search(ctx context.Context, input SearchInput) (SearchResult, error) {
	resolution, err := s.resolve(input.Clients)
	if err != nil {
		return SearchResult{}, err
	}

	rating := s.effectiveRating(input.Rating)
	limit := s.clampLimit(input.Limit, defaultSearchLimit)

	var (
		posts     []booru.Post
		skipped   = resolution.skipped
		warnings  []string
		lastErr   error
		queried   int
		successes int
	)

	for _, entry := range resolution.active {
		if !s.clientAllowed(entry, rating) {
			skipped = append(skipped, booru.Skipped{Client: entry.Name, Reason: noRatingFilterReason})

			continue
		}

		queried++

		clientPosts, err := entry.Provider.Search(ctx, booru.SearchParams{
			Tags:    input.Tags,
			Limit:   limit,
			Page:    input.Page,
			Rating:  rating,
			Exclude: s.opts.BlockedTags,
			Random:  input.Random,
		})
		if err != nil {
			lastErr = err
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name, err))

			continue
		}

		successes++
		posts = append(posts, s.filterPosts(clientPosts, rating)...)
	}

	result := SearchResult{Posts: posts, Skipped: skipped, Warnings: warnings}

	if queried > 0 && successes == 0 && lastErr != nil {
		return result, fmt.Errorf("catalog: all clients failed: %w", lastErr)
	}

	return result, nil
}
