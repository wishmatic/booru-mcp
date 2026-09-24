package catalog

import (
	"context"
	"fmt"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

// exact resolves aliases before looking the name up, so `dickgirl` answers with `futanari` and its count rather than a
// bare zero. An alias that resolves to a missing tag still reports the alias, which is what makes it distinct from a
// genuine miss.
func (s *Service) exact(ctx context.Context, search string, categories []booru.TagCategory) (TagsResult, error) {
	aliasOf, aliased, aliasErr := s.source.AliasTarget(ctx, search)

	lookup := search
	if aliased {
		lookup = aliasOf
	}

	tag, found, err := s.source.FindTag(ctx, lookup)
	if err != nil {
		return TagsResult{}, fmt.Errorf("catalog: %w", err)
	}

	if !found {
		if aliasErr != nil {
			return s.result(search, []booru.Tag{}, false, StatusUnknown, ""), nil
		}

		return s.exactMiss(search, aliased, aliasOf), nil
	}

	if !matchesCategory(categories, tag.Category) {
		return s.exactMiss(search, false, ""), nil
	}

	tag.Implications = s.exactImplications(ctx, tag.Name)

	if !aliased {
		return s.result(search, []booru.Tag{tag}, false, StatusOK, ""), nil
	}

	aliasRow := tag
	aliasRow.Name = search
	aliasRow.AliasOf = aliasOf

	return s.result(search, []booru.Tag{aliasRow, tag}, false, StatusOK, aliasOf), nil
}

func (s *Service) exactMiss(search string, aliased bool, aliasOf string) TagsResult {
	if !aliased || aliasOf == "" {
		return s.result(search, []booru.Tag{}, false, StatusExactNotFound, "")
	}

	aliasRow := booru.Tag{Name: search, Category: booru.CategoryGeneral, AliasOf: aliasOf, Implications: []string{}}

	return s.result(search, []booru.Tag{aliasRow}, false, StatusOK, aliasOf)
}

// exactImplications prefers the local graph but falls back to one upstream call, so an exact answer is never blocked on
// whether the background index has been built yet.
func (s *Service) exactImplications(ctx context.Context, name string) []string {
	if known := knownImplications(s.opts.Relations, name); len(known) > 0 {
		return known
	}

	known, err := s.source.Implications(ctx, name)
	if err != nil {
		return []string{}
	}

	return known
}

func matchesCategory(categories []booru.TagCategory, category booru.TagCategory) bool {
	if len(categories) == 0 {
		return true
	}

	for _, wanted := range categories {
		if wanted == category {
			return true
		}
	}

	return false
}
