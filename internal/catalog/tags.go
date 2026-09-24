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
	StatusOffsetPastEnd    Status = "offset_past_end"
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
	Synonyms     []string
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

	switch {
	case len(tags) > 0:
		status = StatusOK

	case input.Offset > 0 && window.Matched:
		status = StatusOffsetPastEnd

	default:
		status = StatusNoSubstringMatch
	}

	result := s.result(search, tags, window.More, status, "")
	result.Withheld = window.Withheld
	result.WithheldBest = window.WithheldBest
	result.Synonyms = s.synonyms(search, tags)

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

// synonyms surfaces the canonical names the search is also known by, which substring matching alone can never find. It
// is index-only: recall must not add an upstream request to every search, so an unbuilt index simply reports nothing.
func (s *Service) synonyms(search string, tags []booru.Tag) []string {
	if s.opts.Relations == nil || !s.opts.Relations.Ready() {
		return []string{}
	}

	shown := make(map[string]bool, len(tags))

	for _, tag := range tags {
		shown[tag.Name] = true
	}

	out := make([]string, 0)

	for _, name := range s.opts.Relations.Synonyms(search) {
		if name == search || shown[name] || s.blocked[name] {
			continue
		}

		out = append(out, name)
	}

	return out
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
