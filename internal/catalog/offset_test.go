package catalog

import (
	"context"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestTagsOffsetPastEndHasItsOwnStatus(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{}, Matched: true, Withheld: 100, WithheldBest: 0}}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "blue hair", Offset: 1000, Limit: 25})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusOffsetPastEnd {
		t.Fatalf("status = %q, want offset_past_end", result.Status)
	}

	if len(result.Tags) != 0 {
		t.Errorf("tags = %+v, want none", result.Tags)
	}
}

func TestTagsEmptyAtNonZeroOffsetAndUnmatchedIsNoSubstringMatch(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{}}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "qzxwvjklm", Offset: 500, Limit: 25})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusNoSubstringMatch {
		t.Fatalf("status = %q, want no_substring_match for a genuinely unmatched search", result.Status)
	}
}

func TestTagsEmptyOffsetZeroAndUnmatchedIsNoSubstringMatch(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{}}

	result, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "qzxwvjklm", Limit: 25})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if result.Status != StatusNoSubstringMatch {
		t.Fatalf("status = %q, want no_substring_match", result.Status)
	}
}
