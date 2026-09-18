package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"go.uber.org/zap"
)

type stubProvider struct {
	posts   []booru.Post
	post    booru.Post
	tags    []booru.Tag
	popular []booru.Tag
	err     error
}

func (s *stubProvider) Name() string                     { return "danbooru" }
func (s *stubProvider) Capabilities() booru.Capabilities { return booru.Capabilities{Rating: true} }

func (s *stubProvider) Search(context.Context, booru.SearchParams) ([]booru.Post, error) {
	return s.posts, s.err
}

func (s *stubProvider) Post(context.Context, string) (booru.Post, error) {
	if s.err != nil {
		return booru.Post{}, s.err
	}

	return s.post, nil
}

func (s *stubProvider) SearchTags(context.Context, booru.TagQuery) ([]booru.Tag, error) {
	return s.tags, s.err
}

func (s *stubProvider) PopularTags(context.Context, booru.PopularQuery) ([]booru.Tag, error) {
	return s.popular, s.err
}

func newSession(t *testing.T, provider booru.Provider, opts catalog.Options) *mcp.ClientSession {
	t.Helper()

	service := newTestCatalog(t, provider, opts)

	srv, err := New(Deps{
		Log:                   zap.NewNop(),
		Catalog:               service,
		RelatedClients:        []string{"danbooru"},
		RelatedDefaultClients: []string{"danbooru"},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return connectSession(t, srv)
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) error: %v", name, err)
	}

	return result
}

func textOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) == 0 {
		t.Fatalf("result has no content")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want text content", result.Content[0])
	}

	return text.Text
}

func TestTagsCallReturnsCounts(t *testing.T) {
	provider := &stubProvider{tags: []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 900},
		{Name: "blue_sky", Category: booru.CategoryGeneral, Count: 450},
	}}

	session := newSession(t, provider, catalog.Options{MaxLimit: 100, CacheTTL: 24 * time.Hour})
	result := callTool(t, session, "tags", map[string]any{"query": "blue"})

	if result.IsError {
		t.Fatalf("tags returned an error: %s", textOf(t, result))
	}

	text := textOf(t, result)

	if !strings.Contains(text, "blue_eyes") || !strings.Contains(text, "900") || !strings.Contains(text, "100%") {
		t.Errorf("text = %q, want the tag, its work count, and its percentage", text)
	}
}

func TestPopularCallReturnsPerClientBreakdown(t *testing.T) {
	provider := &stubProvider{popular: []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 900},
		{Name: "smile", Category: booru.CategoryGeneral, Count: 450},
	}}

	session := newSession(t, provider, catalog.Options{MaxLimit: 100, CacheTTL: 24 * time.Hour})
	result := callTool(t, session, "popular", map[string]any{})

	if result.IsError {
		t.Fatalf("popular returned an error: %s", textOf(t, result))
	}

	text := textOf(t, result)

	for _, want := range []string{"blue_eyes", "danbooru", "900 works, rank 1, 100%"} {
		if !strings.Contains(text, want) {
			t.Errorf("text = %q, want %q", text, want)
		}
	}
}

func TestGetCallReturnsPermalinkOnly(t *testing.T) {
	provider := &stubProvider{post: booru.Post{
		Client:     "danbooru",
		ID:         "5",
		URL:        "https://danbooru.donmai.us/posts/5",
		FileURL:    "https://cdn.example.com/5.png",
		PreviewURL: "https://cdn.example.com/5-preview.png",
		Rating:     booru.RatingGeneral,
		Tags:       []booru.Tag{{Name: "smile", Category: booru.CategoryGeneral}},
	}}

	session := newSession(t, provider, catalog.Options{MaxLimit: 100})
	result := callTool(t, session, "get", map[string]any{"id": "5"})

	if result.IsError {
		t.Fatalf("get returned an error: %s", textOf(t, result))
	}

	for _, content := range result.Content {
		if _, ok := content.(*mcp.ImageContent); ok {
			t.Fatal("get returned inline image content, want URLs only")
		}
	}

	text := textOf(t, result)

	if !strings.Contains(text, "https://danbooru.donmai.us/posts/5") {
		t.Errorf("text = %q, want the permalink", text)
	}
}

func TestSearchReturnsNoImageContent(t *testing.T) {
	provider := &stubProvider{posts: []booru.Post{{
		Client: "danbooru",
		ID:     "7",
		URL:    "https://danbooru.donmai.us/posts/7",
		Rating: booru.RatingGeneral,
	}}}

	session := newSession(t, provider, catalog.Options{MaxLimit: 100})
	result := callTool(t, session, "search", map[string]any{"tags": "smile"})

	if result.IsError {
		t.Fatalf("search returned an error: %s", textOf(t, result))
	}

	for _, content := range result.Content {
		if _, ok := content.(*mcp.ImageContent); ok {
			t.Fatal("search returned inline image content, want URLs only")
		}
	}
}

func TestHandlerErrorsArePrefixed(t *testing.T) {
	provider := &stubProvider{err: errors.New("boom")}
	session := newSession(t, provider, catalog.Options{MaxLimit: 100})

	result := callTool(t, session, "tags", map[string]any{"query": "blue"})
	if !result.IsError {
		t.Fatal("tags error = nil, want an error result")
	}

	if text := textOf(t, result); !strings.HasPrefix(text, "tags:") {
		t.Errorf("error text = %q, want a tags: prefix", text)
	}
}

func TestRelatedRejectsClientOutsideEnum(t *testing.T) {
	session := newSession(t, &stubProvider{}, catalog.Options{MaxLimit: 10})

	result := callTool(t, session, "related", map[string]any{"tag": "smile", "clients": []string{"yandere"}})
	if !result.IsError {
		t.Fatal("related with a client outside the enum = nil error, want schema validation to reject it")
	}
}

func TestSearchRejectsInvalidRating(t *testing.T) {
	session := newSession(t, &stubProvider{}, catalog.Options{MaxLimit: 100})

	result := callTool(t, session, "search", map[string]any{"tags": "smile", "rating": "nsfw"})

	if !result.IsError {
		t.Fatal("search with an invalid rating = nil error, want an error")
	}
}
