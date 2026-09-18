package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const defaultGetClient = "danbooru"

func (s *Service) Get(ctx context.Context, client, id string) (booru.Post, error) {
	name := strings.TrimSpace(client)
	if name == "" {
		name = defaultGetClient
	}

	entry, err := s.registry.Get(name)
	if err != nil {
		return booru.Post{}, fmt.Errorf("catalog: %w", err)
	}

	if !entry.Active {
		return booru.Post{}, fmt.Errorf("catalog: client %s is inactive: %s", name, entry.Reason)
	}

	post, err := entry.Provider.Post(ctx, id)
	if err != nil {
		return booru.Post{}, fmt.Errorf("catalog: %w", err)
	}

	rating := s.effectiveRating("")
	if !rating.IsUnset() && rating != booru.RatingAll && post.Rating.Rank() > rating.Rank() {
		return booru.Post{}, fmt.Errorf("catalog: post %s is rated %s, above CONTENT_RATING %s", post.Ref(), post.Rating, rating)
	}

	if hasBlockedTag(post, blockedSet(s.opts.BlockedTags)) {
		return booru.Post{}, fmt.Errorf("catalog: post %s carries a blocked tag", post.Ref())
	}

	return post, nil
}
