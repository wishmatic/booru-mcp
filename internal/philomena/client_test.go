package philomena

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
)

type noopLimiter struct{}

func (noopLimiter) Wait(context.Context) error { return nil }

func newTestProvider(t *testing.T, handler http.HandlerFunc, apiKey string) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	return New(Config{BaseURL: server.URL, APIKey: apiKey, MaxLimit: 3, HTTP: httpClient})
}

func TestSearchRequest(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}, "secret-key")

	_, err := client.Search(context.Background(), booru.SearchParams{
		Tags:    "blue eyes",
		Limit:   10,
		Page:    2,
		Rating:  booru.RatingGeneral,
		Exclude: []string{"Bad Tag"},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("q") != "blue,eyes,-bad_tag" {
		t.Errorf("q = %q, want comma-joined terms", got.Get("q"))
	}

	if got.Get("per_page") != "3" {
		t.Errorf("per_page = %q, want it capped", got.Get("per_page"))
	}

	if got.Get("page") != "2" {
		t.Errorf("page = %q, want 2", got.Get("page"))
	}

	if got.Get("key") != "secret-key" {
		t.Errorf("key = %q, want the configured key", got.Get("key"))
	}
}

func TestKeyOnlyWhenConfigured(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}, "")

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("key") != "" {
		t.Errorf("key = %q, want it absent", got.Get("key"))
	}
}

func TestCapabilitiesHaveNoRating(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {}, "")

	if client.Capabilities().Rating {
		t.Error("Capabilities().Rating = true, want false for Philomena")
	}
}

func TestTagsMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tags":[
			{"name":"artist:somebody","images":900},
			{"name":"character:someone","images":800},
			{"name":"species:a_species","images":700},
			{"name":"origin:a_series","images":600},
			{"name":"oc:1234","images":500},
			{"name":"plain_tag","images":400}
		]}`))
	}, "")

	tags, err := client.SearchTags(context.Background(), booru.TagQuery{Query: "some"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := map[string]booru.TagCategory{
		"somebody":  booru.CategoryArtist,
		"someone":   booru.CategoryCharacter,
		"a_species": booru.CategoryCharacter,
		"a_series":  booru.CategoryCopyright,
		"1234":      booru.CategoryMeta,
		"plain_tag": booru.CategoryGeneral,
	}

	for _, tag := range tags {
		if want[tag.Name] != tag.Category {
			t.Errorf("tag %s category = %q, want %q", tag.Name, tag.Category, want[tag.Name])
		}
	}

	if len(tags) != len(want) {
		t.Fatalf("tags = %d, want %d", len(tags), len(want))
	}

	if tags[0].Count != 900 {
		t.Errorf("count = %d, want the images count", tags[0].Count)
	}
}

func TestPostMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"posts":[{
			"id": 5,
			"created_at": "2024-01-02T03:04:05.000-05:00",
			"width": 100,
			"height": 200,
			"score": 7,
			"source": "https://example.com",
			"tags": ["artist:somebody","safe","character:someone"],
			"representations": {
				"full": {"url": "https://cdn.example.com/5-full.png", "width": 100, "height": 200},
				"large": {"url": "https://cdn.example.com/5-large.png"},
				"medium": {"url": "https://cdn.example.com/5-medium.png"}
			}
		}]}`))
	}, "")

	post, err := client.Post(context.Background(), "5")
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}

	if post.URL != client.baseURL+"/images/5" {
		t.Errorf("URL = %q", post.URL)
	}

	if post.Rating != booru.RatingExplicit {
		t.Errorf("Rating = %q, want explicit since the API has no rating field", post.Rating)
	}

	if post.FileURL != "https://cdn.example.com/5-full.png" || post.SampleURL != "https://cdn.example.com/5-large.png" ||
		post.PreviewURL != "https://cdn.example.com/5-medium.png" {
		t.Errorf("representations = %q / %q / %q", post.FileURL, post.SampleURL, post.PreviewURL)
	}

	if post.Width != 100 || post.Height != 200 || post.Score != 7 {
		t.Errorf("post = %+v", post)
	}

	grouped := post.TagsByCategory()

	if len(grouped[booru.CategoryArtist]) != 1 || grouped[booru.CategoryArtist][0] != "somebody" {
		t.Errorf("artist tags = %v", grouped[booru.CategoryArtist])
	}

	if len(grouped[booru.CategoryGeneral]) != 1 || grouped[booru.CategoryGeneral][0] != "safe" {
		t.Errorf("general tags = %v", grouped[booru.CategoryGeneral])
	}
}

func TestPostNotFound(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}, "")

	if _, err := client.Post(context.Background(), "1"); err == nil {
		t.Fatal("Post() error = nil, want an error for an empty result")
	}
}
