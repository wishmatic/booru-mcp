package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
)

type stubRelations struct {
	synonyms []string
}

func (s stubRelations) Implications(string) []string    { return nil }
func (s stubRelations) Synonyms(string) []string        { return s.synonyms }
func (s stubRelations) Canonical(string) (string, bool) { return "", false }
func (s stubRelations) Ready() bool                     { return true }

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

	if !out.Tags[0].CountIsTarget {
		t.Error("count_is_target = false on the alias row, want the target's count flagged")
	}

	if out.Tags[1].CountIsTarget {
		t.Error("count_is_target = true on the canonical row, want it false")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if ok && !strings.Contains(text.Text, "alias of futanari (52264 works there)") {
		t.Errorf("text %q does not attribute the count to the target", text.Text)
	}

	if ok && strings.Contains(text.Text, "dickgirl (general): 52264 works") {
		t.Errorf("text %q reads as if dickgirl has its own work count", text.Text)
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

func TestTagsRendersTheWithheldFilter(t *testing.T) {
	source := &stubSource{page: booru.TagPage{Withheld: 1, WithheldBest: 0}}

	result, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "ass_grabeye_contact"})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if out.Withheld != 1 || out.WithheldBest != 0 || out.MinCount != DefaultMinCount {
		t.Fatalf("output = %+v, want the withheld row reported", out)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want text", result.Content[0])
	}

	if strings.Contains(text.Text, "No tag name contains \"ass_grabeye_contact\".") {
		t.Errorf("text %q still asserts the tag does not exist", text.Text)
	}

	for _, want := range []string{"with at least 1 works", "hidden by the floor", "min_count=0"} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("text %q does not contain %q", text.Text, want)
		}
	}
}

func TestTagsZeroFloorReturnsTheWithheldRow(t *testing.T) {
	source := &stubSource{page: booru.TagPage{Tags: []booru.Tag{
		{Name: "ass_grabeye_contact", Category: booru.CategoryGeneral, Count: 0},
	}}}

	_, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "ass_grabeye_contact", MinCount: intPtr(0)})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if source.query.MinCount != 0 {
		t.Errorf("upstream min count = %d, want 0", source.query.MinCount)
	}

	if len(out.Tags) != 1 || out.Tags[0].Name != "ass_grabeye_contact" || out.Tags[0].Count != 0 {
		t.Fatalf("tags = %+v, want the zero-work row returned", out.Tags)
	}

	if out.Withheld != 0 {
		t.Errorf("withheld = %d, want none at a zero floor", out.Withheld)
	}
}

func TestTagsRendersSynonyms(t *testing.T) {
	opts := catalog.Options{MaxLimit: 100, MaxOffset: 1000, Relations: stubRelations{synonyms: []string{"pee", "urine"}}}

	result, out, err := handlerFor(t, &stubSource{}, opts).
		tags(context.Background(), nil, tagsInput{Search: "piss"})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if len(out.Synonyms) != 2 || out.Synonyms[0] != "pee" {
		t.Fatalf("synonyms = %v, want [pee urine]", out.Synonyms)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "Known aliases: pee, urine.") {
		t.Errorf("text = %#v, want it to name the synonyms", result.Content[0])
	}
}

func TestUnknownStatusThroughSession(t *testing.T) {
	source := &stubSource{aliasErr: errors.New("alias endpoint down")}
	srv := newTestServer(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000})

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "tags",
		Arguments: map[string]any{"search": "maybe_tag", "exact": true},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tags returned an error: %+v", result.Content)
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}

	if !strings.Contains(string(raw), `"status":"unknown"`) {
		t.Errorf("structured content = %s, want status unknown rather than a definitive miss", raw)
	}
}

func TestTagsRendersOffsetPastEnd(t *testing.T) {
	source := &stubSource{page: booru.TagPage{Matched: true, Withheld: 100}}

	result, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "blue hair", Offset: intPtr(1000)})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if out.Status != string(catalog.StatusOffsetPastEnd) {
		t.Fatalf("status = %q, want offset_past_end", out.Status)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want text", result.Content[0])
	}

	if strings.Contains(text.Text, "No tag name contains") {
		t.Errorf("text %q uses the no-match wording for an offset past the end", text.Text)
	}

	for _, want := range []string{"Offset 1000 is past the last result", "no further page"} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("text %q does not contain %q", text.Text, want)
		}
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
