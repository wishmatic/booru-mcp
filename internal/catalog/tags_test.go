package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type fakeSource struct {
	page  booru.TagPage
	err   error
	calls int
	query booru.TagQuery
}

func (f *fakeSource) SearchTags(_ context.Context, query booru.TagQuery) (booru.TagPage, error) {
	f.calls++
	f.query = query

	return f.page, f.err
}

func testOptions() Options {
	return Options{MaxLimit: 100, MaxOffset: 1000}
}

func TestTagsSendsTheCanonicalSearch(t *testing.T) {
	tests := map[string]string{
		"blue hair":    "blue_hair",
		"Blue_Hair":    "blue_hair",
		"  blue  hair": "blue_hair",
		"blue*hair":    "blue_hair",
		"blue?hair":    "blue_hair",
		"*blue?%hair*": "blue_hair",
		"50%":          "50",
		"blue_":        "blue",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			source := &fakeSource{}
			service := New(source, testOptions())

			result, err := service.Tags(context.Background(), TagsInput{Search: input, Limit: 5})
			if err != nil {
				t.Fatalf("Tags() error: %v", err)
			}

			if source.query.Search != want {
				t.Errorf("upstream search = %q, want %q", source.query.Search, want)
			}

			if result.Search != want {
				t.Errorf("result search = %q, want %q", result.Search, want)
			}
		})
	}
}

func TestTagsRejectsSearchWithNothingToMatch(t *testing.T) {
	for _, input := range []string{"", "   ", "*", "??", "%%", `\`, "_"} {
		t.Run(input, func(t *testing.T) {
			source := &fakeSource{}
			service := New(source, testOptions())

			_, err := service.Tags(context.Background(), TagsInput{Search: input, Limit: 5})
			if err == nil {
				t.Fatal("Tags() error = nil, want an error")
			}

			if source.calls != 0 {
				t.Errorf("source calls = %d, want none", source.calls)
			}
		})
	}
}

func TestTagsRejectsWindowOutOfRange(t *testing.T) {
	tests := map[string]struct {
		offset, limit int
		wantNamed     string
	}{
		"negative offset": {offset: -1, limit: 5, wantNamed: "offset"},
		"offset too big":  {offset: 1001, limit: 5, wantNamed: "offset"},
		"zero limit":      {offset: 0, limit: 0, wantNamed: "limit"},
		"limit too big":   {offset: 0, limit: 101, wantNamed: "limit"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			source := &fakeSource{}
			service := New(source, testOptions())

			_, err := service.Tags(context.Background(), TagsInput{Search: "blue", Offset: tt.offset, Limit: tt.limit})
			if err == nil {
				t.Fatal("Tags() error = nil, want an error")
			}

			if !strings.Contains(err.Error(), tt.wantNamed) {
				t.Errorf("error = %q, want it to name %s", err, tt.wantNamed)
			}

			if source.calls != 0 {
				t.Errorf("source calls = %d, want none", source.calls)
			}
		})
	}
}

func TestTagsPassesTheWindowThrough(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{{Name: "blue_hair"}}, More: true}}
	service := New(source, testOptions())

	result, err := service.Tags(context.Background(), TagsInput{Search: "blue", Offset: 30, Limit: 25})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if source.query.Offset != 30 || source.query.Limit != 25 {
		t.Errorf("upstream window = %d/%d, want 30/25", source.query.Offset, source.query.Limit)
	}

	if !result.More || len(result.Tags) != 1 || result.Tags[0].Name != "blue_hair" {
		t.Errorf("result = %+v, want the provider's page", result)
	}
}

func TestTagsDropsBlockedTags(t *testing.T) {
	source := &fakeSource{page: booru.TagPage{Tags: []booru.Tag{
		{Name: "blocked_tag"},
		{Name: "blue_hair"},
	}}}

	opts := testOptions()
	opts.BlockedTags = []string{"blocked_tag"}

	result, err := New(source, opts).Tags(context.Background(), TagsInput{Search: "blue", Limit: 25})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Tags) != 1 || result.Tags[0].Name != "blue_hair" {
		t.Errorf("tags = %+v, want only the unblocked tag", result.Tags)
	}
}

func TestTagsPropagatesProviderError(t *testing.T) {
	source := &fakeSource{err: errors.New("danbooru: boom")}

	_, err := New(source, testOptions()).Tags(context.Background(), TagsInput{Search: "blue", Limit: 25})
	if err == nil {
		t.Fatal("Tags() error = nil, want an error")
	}

	if !errors.Is(err, source.err) || !strings.Contains(err.Error(), "catalog:") {
		t.Errorf("error = %q, want the provider error wrapped", err)
	}
}

func TestNewAppliesDefaultBounds(t *testing.T) {
	service := New(&fakeSource{}, Options{})

	if service.opts.MaxLimit != defaultMaxLimit || service.opts.MaxOffset != defaultMaxOffset {
		t.Errorf("bounds = %d/%d, want the defaults", service.opts.MaxLimit, service.opts.MaxOffset)
	}
}
