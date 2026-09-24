package catalog

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// wildcardMetacharacters are the pattern characters the Danbooru tag API understands. They are replaced with a space
// rather than deleted, so a search like `blue*hair` reads as `blue_hair` instead of `bluehair`.
var wildcardMetacharacters = strings.NewReplacer("*", " ", "?", " ", "%", " ", `\`, " ")

type Status string

const (
	StatusOK               Status = "ok"
	StatusNoSubstringMatch Status = "no_substring_match"
	StatusExactNotFound    Status = "exact_not_found"
	StatusUnknown          Status = "unknown"
)

type TagsInput struct {
	Search     string
	Offset     int
	Limit      int
	MinCount   int
	Exact      bool
	Categories []booru.TagCategory
}

type TagsResult struct {
	Search       string
	Tags         []booru.Tag
	More         bool
	Status       Status
	AliasOf      string
	SnapshotDate string
	Withheld     int
	WithheldBest int
}

func (s *Service) Tags(ctx context.Context, input TagsInput) (TagsResult, error) {
	search, err := normalizeSearch(input.Search)
	if err != nil {
		return TagsResult{}, err
	}

	if err := validateCategories(input.Categories); err != nil {
		return TagsResult{}, err
	}

	if input.Exact {
		return s.exact(ctx, search, input.Categories)
	}

	if err := s.validateWindow(input.Offset, input.Limit); err != nil {
		return TagsResult{}, err
	}

	if input.MinCount < 0 {
		return TagsResult{}, fmt.Errorf("min_count must be zero or greater, got %d", input.MinCount)
	}

	query := booru.TagQuery{
		Search:     search,
		Categories: input.Categories,
		Offset:     input.Offset,
		Limit:      input.Limit,
		MinCount:   input.MinCount,
	}

	window, err := s.source.SearchTags(ctx, query)
	if err != nil {
		return TagsResult{}, fmt.Errorf("catalog: %w", err)
	}

	tags := s.enrich(s.filterBlocked(window.Tags))

	status := StatusOK
	if len(tags) == 0 {
		status = StatusNoSubstringMatch
	}

	result := s.result(search, tags, window.More, status, "")
	result.Withheld = window.Withheld
	result.WithheldBest = window.WithheldBest

	return result, nil
}

func (s *Service) result(search string, tags []booru.Tag, more bool, status Status, aliasOf string) TagsResult {
	return TagsResult{
		Search:       search,
		Tags:         tags,
		More:         more,
		Status:       status,
		AliasOf:      aliasOf,
		SnapshotDate: s.snapshotDate(),
	}
}

func (s *Service) enrich(tags []booru.Tag) []booru.Tag {
	out := make([]booru.Tag, 0, len(tags))

	for _, tag := range tags {
		tag.Implications = knownImplications(s.opts.Relations, tag.Name)
		out = append(out, tag)
	}

	return out
}

func knownImplications(index RelationIndex, name string) []string {
	if index == nil {
		return []string{}
	}

	known := index.Implications(name)
	if len(known) == 0 {
		return []string{}
	}

	return known
}

func validateCategories(categories []booru.TagCategory) error {
	for _, category := range categories {
		if _, ok := booru.ParseTagCategory(category.String()); !ok {
			return fmt.Errorf("category %q is not one of %s", category, booru.CategoryList())
		}
	}

	return nil
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
