package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"go.uber.org/zap"
)

type stubSource struct {
	page  booru.TagPage
	err   error
	calls int
	query booru.TagQuery

	tag          booru.Tag
	found        bool
	findErr      error
	aliasTarget  string
	aliased      bool
	aliasErr     error
	implications []string
	implErr      error
}

func (s *stubSource) SearchTags(_ context.Context, query booru.TagQuery) (booru.TagPage, error) {
	s.calls++
	s.query = query

	return s.page, s.err
}

func (s *stubSource) FindTag(_ context.Context, name string) (booru.Tag, bool, error) {
	s.calls++
	s.query = booru.TagQuery{Search: name}

	return s.tag, s.found, s.findErr
}

func (s *stubSource) AliasTarget(_ context.Context, name string) (string, bool, error) {
	s.calls++
	s.query = booru.TagQuery{Search: name}

	return s.aliasTarget, s.aliased, s.aliasErr
}

func (s *stubSource) Implications(_ context.Context, name string) ([]string, error) {
	s.calls++
	s.query = booru.TagQuery{Search: name}

	return s.implications, s.implErr
}

func newTestServer(t *testing.T, source catalog.TagSource, opts catalog.Options) *mcp.Server {
	t.Helper()

	srv, err := New(Deps{Log: zap.NewNop(), Catalog: catalog.New(source, opts)})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func connectSession(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return session
}

func handlerFor(t *testing.T, source catalog.TagSource, opts catalog.Options) *handlers {
	t.Helper()

	return &handlers{log: zap.NewNop(), catalog: catalog.New(source, opts)}
}

func TestNewRequiresCatalog(t *testing.T) {
	if _, err := New(Deps{Log: zap.NewNop()}); err == nil {
		t.Fatal("New() error = nil, want an error without a catalog")
	}
}

func TestRegistersOnlyTheTagsTool(t *testing.T) {
	srv := newTestServer(t, &stubSource{}, catalog.Options{MaxLimit: 100, MaxOffset: 1000})

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("tools = %d, want exactly one", len(result.Tools))
	}

	if result.Tools[0].Name != "tags" {
		t.Errorf("tool = %q, want tags", result.Tools[0].Name)
	}
}

func TestTagsToolSetsEveryHint(t *testing.T) {
	srv := newTestServer(t, &stubSource{}, catalog.Options{MaxLimit: 100, MaxOffset: 1000})

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	annotations := result.Tools[0].Annotations
	if annotations == nil {
		t.Fatal("annotations = nil, want all four hints set")
	}

	if !annotations.ReadOnlyHint {
		t.Error("readOnlyHint = false, want true")
	}

	if annotations.DestructiveHint == nil || *annotations.DestructiveHint {
		t.Errorf("destructiveHint = %v, want an explicit false", annotations.DestructiveHint)
	}

	if !annotations.IdempotentHint {
		t.Error("idempotentHint = false, want true")
	}

	if annotations.OpenWorldHint == nil || !*annotations.OpenWorldHint {
		t.Errorf("openWorldHint = %v, want an explicit true", annotations.OpenWorldHint)
	}
}

func TestTagsSchemaShape(t *testing.T) {
	srv := newTestServer(t, &stubSource{}, catalog.Options{MaxLimit: 100, MaxOffset: 1000})

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	schema, ok := result.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("input schema = %#v, want an object", result.Tools[0].InputSchema)
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}

	if len(props) != 6 {
		t.Fatalf("properties = %v, want search, offset, limit, min_count, exact, and category", props)
	}

	for _, name := range []string{"search", "offset", "limit", "min_count", "exact", "category"} {
		if _, ok := props[name].(map[string]any); !ok {
			t.Errorf("property %q = %#v, want an object", name, props[name])
		}
	}

	required, ok := schema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "search" {
		t.Errorf("required = %#v, want only search", schema["required"])
	}

	for name, want := range map[string]float64{"offset": 0, "limit": catalog.DefaultTagLimit, "min_count": DefaultMinCount} {
		property, _ := props[name].(map[string]any)
		if got, ok := property["default"].(float64); !ok || got != want {
			t.Errorf("default for %q = %#v, want %v", name, property["default"], want)
		}
	}

	exact, _ := props["exact"].(map[string]any)
	if got, ok := exact["default"].(bool); !ok || got {
		t.Errorf("default for exact = %#v, want false", exact["default"])
	}

	category, _ := props["category"].(map[string]any)
	items, ok := category["items"].(map[string]any)
	if !ok {
		t.Fatalf("category items = %#v, want an items schema", category["items"])
	}

	enum, ok := items["enum"].([]any)
	if !ok || len(enum) != len(booru.CategoryNames()) {
		t.Errorf("category enum = %#v, want every category name", items["enum"])
	}
}

func TestTagsReturnsTheWindow(t *testing.T) {
	source := &stubSource{page: booru.TagPage{
		Tags: []booru.Tag{{Name: "blue_hair", Category: booru.CategoryGeneral, Count: 482913}},
		More: true,
	}}

	result, out, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "blue hair"})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if out.Search != "blue_hair" || out.Offset != 0 || out.Limit != catalog.DefaultTagLimit || !out.More {
		t.Errorf("output = %+v, want the echoed request and more", out)
	}

	if len(out.Tags) != 1 || out.Tags[0].Name != "blue_hair" || out.Tags[0].Category != "general" || out.Tags[0].Count != 482913 {
		t.Errorf("tags = %+v", out.Tags)
	}

	if out.Tags[0].Implications == nil {
		t.Error("implications = nil, want an empty slice")
	}

	if out.Status != string(catalog.StatusOK) || out.SnapshotDate == "" {
		t.Errorf("output = %+v, want an ok status and a snapshot date", out)
	}

	if result == nil || len(result.Content) != 1 {
		t.Fatalf("call result = %+v, want one content entry", result)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want text", result.Content[0])
	}

	for _, want := range []string{"blue_hair (general): 482913 works", "More matches exist; continue with offset 1"} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("text %q does not contain %q", text.Text, want)
		}
	}

	if source.query.Search != "blue_hair" {
		t.Errorf("upstream search = %q, want the canonical form", source.query.Search)
	}
}

func TestTagsRejectsBadInputBeforeCallingUpstream(t *testing.T) {
	tests := map[string]struct {
		input     tagsInput
		wantNamed string
	}{
		"missing search":  {input: tagsInput{}, wantNamed: "search"},
		"blank search":    {input: tagsInput{Search: "  "}, wantNamed: "search"},
		"wildcard search": {input: tagsInput{Search: "*"}, wantNamed: "search"},
		"negative offset": {input: tagsInput{Search: "blue", Offset: intPtr(-1)}, wantNamed: "offset"},
		"huge offset":     {input: tagsInput{Search: "blue", Offset: intPtr(1001)}, wantNamed: "offset"},
		"zero limit":      {input: tagsInput{Search: "blue", Limit: intPtr(0)}, wantNamed: "limit"},
		"huge limit":      {input: tagsInput{Search: "blue", Limit: intPtr(101)}, wantNamed: "limit"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			source := &stubSource{}

			_, _, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
				tags(context.Background(), nil, tt.input)
			if err == nil {
				t.Fatal("tags() error = nil, want an error")
			}

			if !strings.Contains(err.Error(), tt.wantNamed) {
				t.Errorf("error = %q, want it to name %s", err, tt.wantNamed)
			}

			if source.calls != 0 {
				t.Errorf("provider calls = %d, want none", source.calls)
			}
		})
	}
}

func TestTagsDropsBlockedTags(t *testing.T) {
	source := &stubSource{page: booru.TagPage{Tags: []booru.Tag{
		{Name: "blocked_tag"},
		{Name: "blue_hair"},
	}}}

	opts := catalog.Options{MaxLimit: 100, MaxOffset: 1000, BlockedTags: []string{"blocked_tag"}}

	_, out, err := handlerFor(t, source, opts).tags(context.Background(), nil, tagsInput{Search: "blue"})
	if err != nil {
		t.Fatalf("tags() error: %v", err)
	}

	if len(out.Tags) != 1 || out.Tags[0].Name != "blue_hair" {
		t.Errorf("tags = %+v, want only the unblocked tag", out.Tags)
	}
}

func TestTagsSurfacesProviderFailure(t *testing.T) {
	source := &stubSource{err: errors.New("danbooru: upstream is down")}

	_, _, err := handlerFor(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000}).
		tags(context.Background(), nil, tagsInput{Search: "blue"})
	if err == nil {
		t.Fatal("tags() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "tags:") || !strings.Contains(err.Error(), "upstream is down") {
		t.Errorf("error = %q, want it wrapped with the tool name and the cause", err)
	}
}

func TestTagsThroughSession(t *testing.T) {
	source := &stubSource{page: booru.TagPage{Tags: []booru.Tag{
		{Name: "blue_hair", Category: booru.CategoryGeneral, Count: 482913},
	}}}

	srv := newTestServer(t, source, catalog.Options{MaxLimit: 100, MaxOffset: 1000})

	result, err := connectSession(t, srv).CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "tags",
		Arguments: map[string]any{"search": "blue hair", "offset": 0, "limit": 5},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tags returned an error: %+v", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "blue_hair") {
		t.Fatalf("content = %#v, want the tag rendered", result.Content[0])
	}

	if source.query.Offset != 0 || source.query.Limit != 5 {
		t.Errorf("upstream window = %d/%d, want 0/5", source.query.Offset, source.query.Limit)
	}
}

func intPtr(value int) *int {
	return &value
}
