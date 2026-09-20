package catalog

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// wildcardMetacharacters are the pattern characters both tag APIs understand. They are replaced with a space rather
// than deleted, so a search like `blue*hair` reads as `blue_hair` instead of `bluehair`.
var wildcardMetacharacters = strings.NewReplacer("*", " ", "?", " ", "%", " ", `\`, " ")

type TagsInput struct {
	Search string
	Offset int
	Limit  int
}

type TagsResult struct {
	Search string
	Tags   []booru.Tag
	More   bool
}

func (s *Service) Tags(ctx context.Context, input TagsInput) (TagsResult, error) {
	search, err := normalizeSearch(input.Search)
	if err != nil {
		return TagsResult{}, err
	}

	if err := s.validateWindow(input.Offset, input.Limit); err != nil {
		return TagsResult{}, err
	}

	page, err := s.source.SearchTags(ctx, booru.TagQuery{Search: search, Offset: input.Offset, Limit: input.Limit})
	if err != nil {
		return TagsResult{}, fmt.Errorf("catalog: %w", err)
	}

	return TagsResult{Search: search, Tags: s.filterBlocked(page.Tags), More: page.More}, nil
}

// normalizeSearch renders a caller's search as a literal substring of a canonical tag name. The metacharacters both tag
// APIs understand are replaced, so a search can never become an expression, and a search with no letter or digit left
// is rejected rather than sent as a match-everything pattern.
func normalizeSearch(value string) (string, error) {
	search := strings.Trim(booru.NormalizeTag(wildcardMetacharacters.Replace(value)), "_")

	if !hasLetterOrDigit(search) {
		return "", fmt.Errorf("search %q has no letters or digits to match", value)
	}

	return search, nil
}

func hasLetterOrDigit(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}

	return false
}

func (s *Service) validateWindow(offset, limit int) error {
	if offset < 0 || offset > s.opts.MaxOffset {
		return fmt.Errorf("offset must be between 0 and %d, got %d", s.opts.MaxOffset, offset)
	}

	if limit < 1 || limit > s.opts.MaxLimit {
		return fmt.Errorf("limit must be between 1 and %d, got %d", s.opts.MaxLimit, limit)
	}

	return nil
}
