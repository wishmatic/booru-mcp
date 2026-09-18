package moebooru

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
)

type noopLimiter struct{}

func (noopLimiter) Wait(context.Context) error { return nil }

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	return New(Config{BaseURL: server.URL, MaxLimit: 3, HTTP: httpClient})
}

func TestSearchRequest(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := client.Search(context.Background(), booru.SearchParams{
		Tags:    "blue_eyes",
		Limit:   10,
		Page:    2,
		Rating:  booru.RatingQuestionable,
		Exclude: []string{"Bad Tag"},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("tags") != "blue_eyes rating:questionable -bad_tag" {
		t.Errorf("tags = %q", got.Get("tags"))
	}

	if got.Get("limit") != "3" {
		t.Errorf("limit = %q, want it capped", got.Get("limit"))
	}

	if got.Get("page") != "2" {
		t.Errorf("page = %q, want 2", got.Get("page"))
	}
}

func TestSearchTagsMapping(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[
			{"name":"blue_eyes","count":900,"type":0},
			{"name":"somebody","count":500,"type":1},
			{"name":"a_studio","count":400,"type":2},
			{"name":"a_circle","count":300,"type":5},
			{"name":"a_series","count":200,"type":3},
			{"name":"a_character","count":100,"type":4},
			{"name":"odd_type","count":10,"type":99}
		]`))
	})

	tags, err := client.SearchTags(context.Background(), booru.TagQuery{Query: "a", Limit: 10})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if got.Get("name") != "*a*" {
		t.Errorf("name = %q, want *a*", got.Get("name"))
	}

	if got.Get("order") != "count" {
		t.Errorf("order = %q, want count", got.Get("order"))
	}

	want := map[string]booru.TagCategory{
		"blue_eyes":   booru.CategoryGeneral,
		"somebody":    booru.CategoryArtist,
		"a_studio":    booru.CategoryArtist,
		"a_circle":    booru.CategoryCopyright,
		"a_series":    booru.CategoryCopyright,
		"a_character": booru.CategoryCharacter,
		"odd_type":    booru.CategoryGeneral,
	}

	for _, tag := range tags {
		if want[tag.Name] != tag.Category {
			t.Errorf("tag %s category = %q, want %q", tag.Name, tag.Category, want[tag.Name])
		}
	}
}

func TestPopularTags(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[{"name":"blue_eyes","count":900,"type":0}]`))
	})

	tags, err := client.PopularTags(context.Background(), booru.PopularQuery{Limit: 1})
	if err != nil {
		t.Fatalf("PopularTags() error: %v", err)
	}

	if got.Get("order") != "count" || got.Get("name") != "" {
		t.Errorf("query = %v, want count ordering and no name filter", got)
	}

	if len(tags) != 1 || tags[0].Count != 900 {
		t.Errorf("tags = %+v", tags)
	}
}

func TestPostMapping(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[{
			"id": 12,
			"created_at": 1700000000,
			"score": 9,
			"width": 300,
			"height": 200,
			"rating": "e",
			"source": "https://example.com",
			"file_url": "https://cdn.example.com/12.png",
			"sample_url": "https://cdn.example.com/12-sample.png",
			"preview_url": "https://cdn.example.com/12-preview.png",
			"tags": "blue_eyes smile"
		}]`))
	})

	post, err := client.Post(context.Background(), "12")
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}

	if got.Get("tags") != "id:12" {
		t.Errorf("tags = %q, want id:12", got.Get("tags"))
	}

	if post.URL != client.baseURL+"/post/show/12" {
		t.Errorf("URL = %q", post.URL)
	}

	if post.Rating != booru.RatingExplicit || post.Score != 9 {
		t.Errorf("post = %+v", post)
	}

	if post.CreatedAt.IsZero() {
		t.Error("CreatedAt = zero, want the epoch parsed")
	}
}

func TestSearchSendsRandomOrdering(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[]`))
	})

	if !client.Capabilities().Random {
		t.Error("Capabilities().Random = false, want Moebooru random ordering advertised")
	}

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "cat_ears", Random: true}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("tags") != "cat_ears order:random" {
		t.Errorf("tags = %q, want order:random appended", got.Get("tags"))
	}
}

func TestHTMLBodyIsATypedError(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>nope</html>`))
	})

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	var bodyErr *fetch.BodyError
	if !errors.As(err, &bodyErr) {
		t.Fatalf("error = %v, want a typed *fetch.BodyError", err)
	}

	if !strings.Contains(err.Error(), "HTML") || !strings.Contains(err.Error(), "/post.json") {
		t.Errorf("error = %q, want it to identify HTML and the path", err.Error())
	}
}

func TestPostNotFound(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.Post(context.Background(), "1"); err == nil {
		t.Fatal("Post() error = nil, want an error for an empty result")
	}
}
