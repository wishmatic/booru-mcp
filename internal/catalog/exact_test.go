package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type stubRelations struct {
	byName map[string][]string
}

func (s stubRelations) Implications(name string) []string {
	return s.byName[name]
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, time.September, 24, 12, 0, 0, 0, time.UTC) }
}

func TestTagsExactResolvesAlias(t *testing.T) {
	source := &fakeSource{
		aliasTarget: "futanari",
		aliased:     true,
		tag:         booru.Tag{Name: "futanari", Category: booru.CategoryGeneral, Count: 52264},
		found:       true,
	}

	opts := testOptions()
	opts.Relations = stubRelations{byName: map[string][]string{"futanari": {"futanari"}}}
	opts.Clock = fixedClock()

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "dickgirl", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusOK || result.AliasOf != "futanari" {
		t.Fatalf("result = %+v, want an ok alias resolution", result)
	}

	if len(result.Tags) != 2 {
		t.Fatalf("tags = %+v, want the alias row and the canonical row", result.Tags)
	}

	alias := result.Tags[0]
	if alias.Name != "dickgirl" || alias.AliasOf != "futanari" || alias.Count != 52264 || alias.Category != booru.CategoryGeneral {
		t.Errorf("alias row = %+v, want the alias carrying the target's count", alias)
	}

	if result.Tags[1].Name != "futanari" || result.Tags[1].AliasOf != "" {
		t.Errorf("canonical row = %+v, want futanari", result.Tags[1])
	}
}

func TestTagsExactNotFound(t *testing.T) {
	result, err := New(&fakeSource{}, testOptions()).Tags(context.Background(), TagsInput{Search: "qzxwvjklm", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusExactNotFound || len(result.Tags) != 0 {
		t.Fatalf("result = %+v, want exact_not_found with no tags", result)
	}
}

func TestTagsExactAliasToMissingTagStillReportsAlias(t *testing.T) {
	source := &fakeSource{aliasTarget: "ghost_tag", aliased: true}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "old_tag", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusOK || result.AliasOf != "ghost_tag" || len(result.Tags) != 1 {
		t.Fatalf("result = %+v, want the alias reported rather than a bare miss", result)
	}

	if result.Tags[0].AliasOf != "ghost_tag" {
		t.Errorf("alias row = %+v, want alias_of ghost_tag", result.Tags[0])
	}
}

func TestTagsExactUnknownWhenAliasLookupFails(t *testing.T) {
	source := &fakeSource{aliasErr: errors.New("alias endpoint down")}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "maybe", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusUnknown || len(result.Tags) != 0 {
		t.Fatalf("result = %+v, want unknown, not a definitive miss", result)
	}
}

func TestTagsExactReturnsImplications(t *testing.T) {
	source := &fakeSource{
		tag:          booru.Tag{Name: "futanari_pov", Category: booru.CategoryGeneral, Count: 1427},
		found:        true,
		implications: []string{"futanari", "pov"},
	}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "futanari_pov", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Tags) != 1 {
		t.Fatalf("tags = %+v, want one tag", result.Tags)
	}

	got := result.Tags[0].Implications
	if len(got) != 2 || got[0] != "futanari" || got[1] != "pov" {
		t.Errorf("implications = %v, want [futanari pov]", got)
	}
}

func TestTagsExactPrefersTheRelationIndex(t *testing.T) {
	source := &fakeSource{
		tag:     booru.Tag{Name: "futanari_pov", Count: 1427},
		found:   true,
		implErr: errors.New("the upstream call should not happen"),
	}

	opts := testOptions()
	opts.Relations = stubRelations{byName: map[string][]string{"futanari_pov": {"futanari", "pov"}}}

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "futanari_pov", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if got := result.Tags[0].Implications; len(got) != 2 {
		t.Errorf("implications = %v, want the indexed pair", got)
	}
}

func TestTagsExactRespectsCategoryFilter(t *testing.T) {
	source := &fakeSource{tag: booru.Tag{Name: "some_artist", Category: booru.CategoryArtist, Count: 10}, found: true}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{
		Search:     "some_artist",
		Exact:      true,
		Categories: []booru.TagCategory{booru.CategoryGeneral},
	})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusExactNotFound {
		t.Errorf("status = %q, want exact_not_found when the category is filtered out", result.Status)
	}
}

func TestTagsSubstringReportsNoMatch(t *testing.T) {
	result, err := New(&fakeSource{}, testOptions()).Tags(context.Background(), TagsInput{Search: "qzxwvjklm", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusNoSubstringMatch || len(result.Tags) != 0 {
		t.Fatalf("result = %+v, want no_substring_match", result)
	}
}

func TestTagsSubstringEnrichesFromTheIndex(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{
		{Name: "futanari_pov", Count: 1427},
		{Name: "solo", Count: 5},
	}}}

	opts := testOptions()
	opts.Relations = stubRelations{byName: map[string][]string{"futanari_pov": {"futanari", "pov"}}}

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "futanari", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}

	if got := result.Tags[0].Implications; len(got) != 2 {
		t.Errorf("implications = %v, want the indexed pair", got)
	}

	if result.Tags[1].Implications == nil || len(result.Tags[1].Implications) != 0 {
		t.Errorf("implications = %v, want an empty slice, never nil", result.Tags[1].Implications)
	}
}

func TestTagsSnapshotDate(t *testing.T) {
	opts := testOptions()
	opts.Clock = fixedClock()

	result, err := New(&fakeSource{}, opts).Tags(context.Background(), TagsInput{Search: "blue", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.SnapshotDate != "2026-09-24" {
		t.Errorf("snapshot date = %q, want 2026-09-24", result.SnapshotDate)
	}
}

func TestTagsRejectsUnknownCategory(t *testing.T) {
	source := &fakeSource{}

	_, err := New(source, testOptions()).Tags(context.Background(), TagsInput{
		Search:     "blue",
		Limit:      5,
		Categories: []booru.TagCategory{"nonsense"},
	})
	if err == nil {
		t.Fatal("Tags() error = nil, want an unknown category rejected")
	}

	if source.calls != 0 {
		t.Errorf("source calls = %d, want none", source.calls)
	}
}

func TestTagsPassesCategoriesUpstream(t *testing.T) {
	source := &fakeSource{}

	if _, err := New(source, testOptions()).Tags(context.Background(), TagsInput{
		Search:     "piss",
		Limit:      5,
		Categories: []booru.TagCategory{booru.CategoryGeneral},
	}); err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(source.query.Categories) != 1 || source.query.Categories[0] != booru.CategoryGeneral {
		t.Errorf("categories = %v, want [general]", source.query.Categories)
	}
}
