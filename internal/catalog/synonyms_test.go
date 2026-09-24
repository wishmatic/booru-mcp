package catalog

import (
	"context"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func relationsWithSynonyms() stubRelations {
	return stubRelations{
		byName:    map[string][]string{},
		synonyms:  map[string][]string{"piss": {"pee", "urine"}},
		canonical: map[string]string{"piss": "pee"},
	}
}

func TestTagsSubstringSurfacesSynonyms(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{{Name: "piss_bottle", Count: 73}}}}

	opts := testOptions()
	opts.Relations = relationsWithSynonyms()

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "piss", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Synonyms) != 2 || result.Synonyms[0] != "pee" || result.Synonyms[1] != "urine" {
		t.Errorf("synonyms = %v, want [pee urine]", result.Synonyms)
	}
}

func TestTagsSynonymsDropNamesAlreadyShown(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{{Name: "pee", Count: 14212}}}}

	opts := testOptions()
	opts.Relations = relationsWithSynonyms()

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "piss", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Synonyms) != 1 || result.Synonyms[0] != "urine" {
		t.Errorf("synonyms = %v, want only the name not already returned", result.Synonyms)
	}
}

func TestTagsSynonymsHonourBlockedTags(t *testing.T) {
	source := &fakeSource{}

	opts := testOptions()
	opts.Relations = relationsWithSynonyms()
	opts.BlockedTags = []string{"urine"}

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "piss", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Synonyms) != 1 || result.Synonyms[0] != "pee" {
		t.Errorf("synonyms = %v, want the blocked name dropped", result.Synonyms)
	}
}

func TestTagsWithoutIndexHasNoSynonyms(t *testing.T) {
	source := &fakeSource{}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "piss", Limit: 5})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Synonyms == nil || len(result.Synonyms) != 0 {
		t.Errorf("synonyms = %v, want an empty slice", result.Synonyms)
	}
}

func TestTagsExactUsesTheIndexCanonical(t *testing.T) {
	source := &fakeSource{tag: booru.Tag{Name: "pee", Count: 14212}, found: true}

	opts := testOptions()
	opts.Relations = relationsWithSynonyms()

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "piss", Exact: true})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.AliasOf != "pee" || len(result.Tags) != 2 || result.Tags[0].Name != "piss" {
		t.Fatalf("result = %+v, want the alias resolved from the index without an upstream call", result)
	}

	if source.calls != 1 {
		t.Errorf("source calls = %d, want only the tag lookup", source.calls)
	}
}
