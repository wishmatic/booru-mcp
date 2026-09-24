package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestTagsRejectsNegativeMinCount(t *testing.T) {
	source := &fakeSource{}

	_, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "blue", Limit: 5, MinCount: -1})
	if err == nil || !strings.Contains(err.Error(), "min_count") {
		t.Fatalf("Tags() error = %v, want it to name min_count", err)
	}

	if source.calls != 0 {
		t.Errorf("source calls = %d, want none", source.calls)
	}
}

func TestTagsPassesMinCountUpstream(t *testing.T) {
	source := &fakeSource{}

	if _, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "blue", Limit: 5, MinCount: 0}); err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if source.query.MinCount != 0 {
		t.Errorf("upstream min count = %d, want 0", source.query.MinCount)
	}
}

func TestTagsSurfacesWithheld(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{}, Withheld: 1, WithheldBest: 0}}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "ass_grabeye_contact", Limit: 5, MinCount: 1})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusNoSubstringMatch {
		t.Fatalf("status = %q, want no_substring_match", result.Status)
	}

	if result.Withheld != 1 || result.WithheldBest != 0 {
		t.Errorf("withheld = %d best = %d, want 1 and 0", result.Withheld, result.WithheldBest)
	}
}
