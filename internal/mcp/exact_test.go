package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
)

func boolPtr(value bool) *bool {
	return &value
}

func TestTagsRejectsUnknownCategory(t *testing.T) {
	source := &stubSource{}

	_, _, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "piss", Categories: []string{"nonsense"}})
	if err == nil {
		t.Fatal("tags() error = nil, want an unknown category rejected")
	}

	if !strings.Contains(err.Error(), "general") {
		t.Errorf("error = %q, want it to name the valid categories", err)
	}

	if source.calls != 0 {
		t.Errorf("provider calls = %d, want none", source.calls)
	}
}

func TestTagsPassesCategoriesThrough(t *testing.T) {
	source := &stubSource{}

	_, _, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "piss", Categories: []string{"general", "artists"}})
	if err == nil {
		t.Fatal("tags() error = nil, want the invalid category rejected")
	}
}

func TestTagsPassesValidCategoriesThrough(t *testing.T) {
	source := &stubSource{}

	_, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "piss", Categories: []string{"general", "artist"}})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if len(source.query.Categories) != 2 {
		t.Fatalf("upstream categories = %v, want two", source.query.Categories)
	}

	if out.Tags == nil {
		t.Error("tags = nil, want an empty slice")
	}
}

func TestTagsExactPassesThrough(t *testing.T) {
	source := &stubSource{tag: booru.Tag{Name: "1girl", Category: booru.CategoryGeneral, Count: 8455477}, found: true}

	_, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "1girl", Exact: boolPtr(true)})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if !out.Exact || out.Status != string(catalog.StatusOK) {
		t.Fatalf("output = %+v, want an exact ok answer", out)
	}

	if len(out.Tags) != 1 || out.Tags[0].Name != "1girl" || out.Tags[0].Count != 8455477 {
		t.Errorf("tags = %+v, want the single exact tag", out.Tags)
	}

	if source.query.Search != "1girl" {
		t.Errorf("lookup = %q, want 1girl", source.query.Search)
	}
}

func TestTagsExactAliasIsReported(t *testing.T) {
	source := &stubSource{
		aliasTarget: "futanari",
		aliased:     true,
		tag:         booru.Tag{Name: "futanari", Category: booru.CategoryGeneral, Count: 52264},
		found:       true,
	}

	result, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "dickgirl", Exact: boolPtr(true)})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if out.AliasOf != "futanari" || out.Status != string(catalog.StatusOK) {
		t.Fatalf("output = %+v, want alias_of futanari", out)
	}

	if len(out.Tags) != 2 || out.Tags[0].AliasOf != "futanari" || out.Tags[0].Count != 52264 {
		t.Errorf("tags = %+v, want the alias with the target's count", out.Tags)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if ok && !strings.Contains(text.Text, "[alias of futanari]") {
		t.Errorf("text %q does not name the alias target", text.Text)
	}
}

func TestTagsRendersImplications(t *testing.T) {
	source := &stubSource{
		tag:          booru.Tag{Name: "futanari_pov", Category: booru.CategoryGeneral, Count: 1427},
		found:        true,
		implications: []string{"futanari", "pov"},
	}

	result, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "futanari_pov", Exact: boolPtr(true)})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if len(out.Tags[0].Implications) != 2 {
		t.Fatalf("implications = %v, want two", out.Tags[0].Implications)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if ok && !strings.Contains(text.Text, "[implies futanari, pov]") {
		t.Errorf("text %q does not name the implications", text.Text)
	}
}

func TestTagsRendersHonestEmpties(t *testing.T) {
	tests := map[string]struct {
		source   *stubSource
		input    tagsInput
		wantText string
		wantStat catalog.Status
	}{
		"substring miss": {
			source:   &stubSource{},
			input:    tagsInput{Search: "qzxwvjklm"},
			wantText: `No tag name contains "qzxwvjklm".`,
			wantStat: catalog.StatusNoSubstringMatch,
		},
		"exact miss": {
			source:   &stubSource{},
			input:    tagsInput{Search: "no_such_tag", Exact: boolPtr(true)},
			wantText: `No tag named "no_such_tag" exists.`,
			wantStat: catalog.StatusExactNotFound,
		},
		"unknown": {
			source:   &stubSource{aliasErr: errors.New("down")},
			input:    tagsInput{Search: "maybe_tag", Exact: boolPtr(true)},
			wantText: "Could not confirm",
			wantStat: catalog.StatusUnknown,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, out, err := handlerFor(t, tt.source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
				tags(context.Background(), nil, tt.input)
			if err != nil {
				t.Fatalf("tags() error: %v", err)
			}

			if out.Status != string(tt.wantStat) {
				t.Errorf("status = %q, want %q", out.Status, tt.wantStat)
			}

			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok || !strings.Contains(text.Text, tt.wantText) {
				t.Errorf("text = %#v, want it to contain %q", result.Content[0], tt.wantText)
			}
		})
	}
}
