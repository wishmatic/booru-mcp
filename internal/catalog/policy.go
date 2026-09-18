package catalog

import "github.com/wishmatic/booru-mcp/internal/booru"

const noRatingFilterReason = "the client has no rating filter, so it was excluded by CONTENT_RATING"

// effectiveRating narrows a per-call rating to the configured cap. A call can never widen the cap, and an unset call
// means the cap alone.
func (s *Service) effectiveRating(call booru.Rating) booru.Rating {
	cap := s.opts.ContentRating

	if cap.IsUnset() || cap == booru.RatingAll {
		return call
	}

	if call.IsUnset() || call.Rank() > cap.Rank() {
		return cap
	}

	return call
}

func (s *Service) clientAllowed(entry booru.Entry, rating booru.Rating) bool {
	if rating.IsUnset() || rating == booru.RatingAll {
		return true
	}

	if entry.Provider.Capabilities().Rating {
		return true
	}

	return rating.Rank() >= booru.RatingExplicit.Rank()
}

func (s *Service) filterPosts(posts []booru.Post, rating booru.Rating) []booru.Post {
	blocked := blockedSet(s.opts.BlockedTags)
	out := make([]booru.Post, 0, len(posts))

	for _, post := range posts {
		if !rating.IsUnset() && rating != booru.RatingAll && post.Rating.Rank() > rating.Rank() {
			continue
		}

		if hasBlockedTag(post, blocked) {
			continue
		}

		out = append(out, post)
	}

	return out
}

func (s *Service) filterTagNames(tags []booru.FusedTag) []booru.FusedTag {
	blocked := blockedSet(s.opts.BlockedTags)
	out := make([]booru.FusedTag, 0, len(tags))

	for _, tag := range tags {
		if blocked[tag.Name] {
			continue
		}

		out = append(out, tag)
	}

	return out
}

func hasBlockedTag(post booru.Post, blocked map[string]bool) bool {
	if len(blocked) == 0 {
		return false
	}

	for _, tag := range post.Tags {
		if blocked[tag.Name] {
			return true
		}
	}

	return false
}
