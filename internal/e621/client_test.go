package e621

import (
	"context"
	"encoding/base64"
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

func newTestProvider(t *testing.T, handler http.HandlerFunc, login, apiKey string) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, UserAgent: "booru-mcp-test contact@example.com", Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	return New(Config{BaseURL: server.URL, Login: login, APIKey: apiKey, MaxLimit: 3, HTTP: httpClient})
}

func TestSearchRequest(t *testing.T) {
	var (
		gotQuery  url.Values
		gotHeader http.Header
	)

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		gotHeader = r.Header.Clone()
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}, "me@example.com", "secret-key")

	_, err := client.Search(context.Background(), booru.SearchParams{
		Tags:    "blue_eyes",
		Limit:   10,
		Page:    2,
		Rating:  booru.RatingExplicit,
		Exclude: []string{"Bad Tag"},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if gotQuery.Get("tags") != "blue_eyes rating:e -bad_tag" {
		t.Errorf("tags = %q", gotQuery.Get("tags"))
	}

	if gotQuery.Get("limit") != "3" || gotQuery.Get("page") != "2" {
		t.Errorf("limit=%q page=%q", gotQuery.Get("limit"), gotQuery.Get("page"))
	}

	if gotQuery.Get("api_key") != "" {
		t.Errorf("api_key = %q, want credentials in a header, not the URL", gotQuery.Get("api_key"))
	}

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("me@example.com:secret-key"))
	if got := gotHeader.Get("Authorization"); got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}

	if ua := gotHeader.Get("User-Agent"); !strings.Contains(ua, "contact@example.com") {
		t.Errorf("User-Agent = %q, want the configured descriptive value", ua)
	}
}

func TestNoAuthHeaderWhenUnconfigured(t *testing.T) {
	var gotHeader http.Header

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		_, _ = w.Write([]byte(`{"posts":[]}`))
	}, "", "")

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got := gotHeader.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want it absent", got)
	}
}

func TestForbiddenNamesUserAgent(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}, "", "")

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "USER_AGENT") {
		t.Errorf("error = %q, want it to name USER_AGENT", err)
	}
}

func TestTagsMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tags":[
			{"name":"blue_eyes","category":0,"post_count":900},
			{"name":"somebody","category":1,"post_count":800},
			{"name":"a_series","category":3,"post_count":700},
			{"name":"a_character","category":4,"post_count":600},
			{"name":"a_species","category":5,"post_count":500},
			{"name":"an_invalid","category":6,"post_count":400},
			{"name":"a_meta","category":7,"post_count":300},
			{"name":"a_lore","category":8,"post_count":200}
		]}`))
	}, "", "")

	tags, err := client.SearchTags(context.Background(), booru.TagQuery{Query: "a"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := map[string]booru.TagCategory{
		"blue_eyes":   booru.CategoryGeneral,
		"somebody":    booru.CategoryArtist,
		"a_series":    booru.CategoryCopyright,
		"a_character": booru.CategoryCharacter,
		"a_species":   booru.CategoryCharacter,
		"an_invalid":  booru.CategoryGeneral,
		"a_meta":      booru.CategoryMeta,
		"a_lore":      booru.CategoryMeta,
	}

	for _, tag := range tags {
		if want[tag.Name] != tag.Category {
			t.Errorf("tag %s category = %q, want %q", tag.Name, tag.Category, want[tag.Name])
		}
	}
}

func TestPostMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"post":{
			"id": 99,
			"created_at": "2024-01-02T03:04:05.000-05:00",
			"rating": "s",
			"score": {"total": 21},
			"fav_count": 8,
			"sources": ["https://example.com/a", "https://example.com/b"],
			"file": {"url": "https://cdn.example.com/99.png", "width": 640, "height": 480},
			"preview": {"url": "https://cdn.example.com/99-preview.png"},
			"sample": {"url": "https://cdn.example.com/99-sample.png"},
			"tags": {
				"general": ["blue_eyes"],
				"artist": ["somebody"],
				"character": ["someone"],
				"species": ["a_species"],
				"copyright": ["a_series"],
				"meta": ["highres"],
				"lore": ["a_lore"],
				"invalid": ["odd"]
			}
		}}`))
	}, "", "")

	post, err := client.Post(context.Background(), "99")
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}

	if post.URL != client.baseURL+"/posts/99" {
		t.Errorf("URL = %q", post.URL)
	}

	if post.Rating != booru.RatingGeneral || post.Score != 21 || post.FavCount != 8 {
		t.Errorf("post = %+v", post)
	}

	if post.Width != 640 || post.Height != 480 {
		t.Errorf("dimensions = %dx%d", post.Width, post.Height)
	}

	if post.Source != "https://example.com/a; https://example.com/b" {
		t.Errorf("Source = %q", post.Source)
	}

	grouped := post.TagsByCategory()

	if len(grouped[booru.CategoryCharacter]) != 2 {
		t.Errorf("character/species tags = %v, want 2", grouped[booru.CategoryCharacter])
	}

	if len(grouped[booru.CategoryMeta]) != 2 {
		t.Errorf("meta/lore tags = %v, want 2", grouped[booru.CategoryMeta])
	}
}

func TestRelatedTagsAcceptsArrayAndObject(t *testing.T) {
	shapes := map[string]string{
		"array":  `[{"tag":"blue_eyes","frequency":900,"co_occurrence":800}]`,
		"object": `{"related_tags":[{"tag":"blue_eyes","frequency":900,"co_occurrence":800}]}`,
	}

	for name, body := range shapes {
		t.Run(name, func(t *testing.T) {
			client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}, "", "")

			tags, err := client.RelatedTags(context.Background(), booru.RelatedQuery{Tag: "smile"})
			if err != nil {
				t.Fatalf("RelatedTags() error: %v", err)
			}

			if len(tags) != 1 || tags[0].Tag != "blue_eyes" || tags[0].Score != 900 || tags[0].Rank != 1 {
				t.Errorf("tags = %+v", tags)
			}
		})
	}
}
