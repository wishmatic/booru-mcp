package danbooru

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

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, UserAgent: "test", Limiter: noopLimiter{}})
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
		Tags:    "blue_eyes smile",
		Limit:   10,
		Page:    2,
		Rating:  booru.RatingExplicit,
		Exclude: []string{"Bad Tag"},
		Random:  true,
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("tags") != "blue_eyes smile rating:e -bad_tag order:random" {
		t.Errorf("tags = %q", got.Get("tags"))
	}

	if got.Get("limit") != "3" {
		t.Errorf("limit = %q, want it capped at MaxLimit", got.Get("limit"))
	}

	if got.Get("page") != "2" {
		t.Errorf("page = %q, want 2", got.Get("page"))
	}
}

func TestSearchMapsPosts(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{
			"id": 42,
			"created_at": "2024-01-02T03:04:05.000-05:00",
			"rating": "q",
			"score": 12,
			"fav_count": 3,
			"source": "https://example.com",
			"file_url": "https://cdn.example.com/42.png",
			"large_file_url": "https://cdn.example.com/42-large.png",
			"preview_file_url": "https://cdn.example.com/42-preview.png",
			"image_width": 800,
			"image_height": 600,
			"tag_string_general": "blue_eyes smile",
			"tag_string_artist": "somebody",
			"tag_string_character": "someone",
			"tag_string_copyright": "a_series",
			"tag_string_meta": "highres"
		}]`))
	})

	posts, err := client.Search(context.Background(), booru.SearchParams{Tags: "blue_eyes", Limit: 1})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(posts))
	}

	post := posts[0]

	if post.ID != "42" || post.Client != "danbooru" {
		t.Errorf("post = %+v, want id 42 on danbooru", post)
	}

	if post.URL != client.baseURL+"/posts/42" {
		t.Errorf("URL = %q, want the permalink", post.URL)
	}

	if post.Rating != booru.RatingQuestionable {
		t.Errorf("Rating = %q, want questionable", post.Rating)
	}

	if post.Width != 800 || post.Height != 600 || post.Score != 12 || post.FavCount != 3 {
		t.Errorf("post numbers = %+v", post)
	}

	grouped := post.TagsByCategory()

	if len(grouped[booru.CategoryGeneral]) != 2 || len(grouped[booru.CategoryArtist]) != 1 ||
		len(grouped[booru.CategoryCharacter]) != 1 || len(grouped[booru.CategoryCopyright]) != 1 ||
		len(grouped[booru.CategoryMeta]) != 1 {
		t.Errorf("grouped tags = %+v", grouped)
	}
}

func TestSearchTagsRequest(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[{"name":"blue_eyes","category":0,"post_count":900}]`))
	})

	tags, err := client.SearchTags(context.Background(), booru.TagQuery{
		Query:    "blue",
		Category: booru.CategoryArtist,
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if got.Get("search[name_matches]") != "*blue*" {
		t.Errorf("name_matches = %q, want *blue*", got.Get("search[name_matches]"))
	}

	if got.Get("search[order]") != "count" {
		t.Errorf("order = %q, want count", got.Get("search[order]"))
	}

	if got.Get("search[category]") != "1" {
		t.Errorf("category = %q, want 1", got.Get("search[category]"))
	}

	if len(tags) != 1 || tags[0].Count != 900 || tags[0].Category != booru.CategoryGeneral {
		t.Errorf("tags = %+v", tags)
	}
}

func TestPopularTags(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`[{"name":"blue_eyes","category":0,"post_count":900},{"name":"smile","category":0,"post_count":500}]`))
	})

	tags, err := client.PopularTags(context.Background(), booru.PopularQuery{Limit: 2})
	if err != nil {
		t.Fatalf("PopularTags() error: %v", err)
	}

	if got.Get("search[name_matches]") != "" {
		t.Errorf("name_matches = %q, want it absent for popular tags", got.Get("search[name_matches]"))
	}

	if got.Get("search[order]") != "count" {
		t.Errorf("order = %q, want count", got.Get("search[order]"))
	}

	if len(tags) != 2 || tags[0].Name != "blue_eyes" || tags[0].Count != 900 {
		t.Errorf("tags = %+v", tags)
	}
}

func TestRelatedTags(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"query":"smile","related_tags":[
			{"tag":"blue_eyes","frequency":900,"co_occurrence":800},
			{"tag":"red_hair","frequency":100,"co_occurrence":90}
		]}`))
	})

	tags, err := client.RelatedTags(context.Background(), booru.RelatedQuery{Tag: "smile"})
	if err != nil {
		t.Fatalf("RelatedTags() error: %v", err)
	}

	if len(tags) != 2 || tags[0].Tag != "blue_eyes" || tags[0].Score != 900 || tags[0].Rank != 1 ||
		tags[1].Rank != 2 || tags[0].Client != "danbooru" {
		t.Errorf("tags = %+v", tags)
	}
}

func TestPost(t *testing.T) {
	var gotPath string

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":5,"rating":"g","tag_string_general":"smile"}`))
	})

	post, err := client.Post(context.Background(), "5")
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}

	if gotPath != "/posts/5.json" {
		t.Errorf("path = %q, want /posts/5.json", gotPath)
	}

	if post.URL != client.baseURL+"/posts/5" {
		t.Errorf("URL = %q", post.URL)
	}
}

func TestAuthOnlyWhenBothConfigured(t *testing.T) {
	tests := []struct {
		name               string
		login, key         string
		wantLogin, wantKey bool
	}{
		{name: "both", login: "me", key: "secret", wantLogin: true, wantKey: true},
		{name: "login only", login: "me"},
		{name: "key only", key: "secret"},
		{name: "neither"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got url.Values

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(server.Close)

			httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, Limiter: noopLimiter{}})
			if err != nil {
				t.Fatalf("fetch.New() error: %v", err)
			}

			client := New(Config{BaseURL: server.URL, Login: tt.login, APIKey: tt.key, HTTP: httpClient})

			if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"}); err != nil {
				t.Fatalf("Search() error: %v", err)
			}

			if (got.Get("login") != "") != tt.wantLogin || (got.Get("api_key") != "") != tt.wantKey {
				t.Errorf("login=%q api_key=%q, want login=%v key=%v", got.Get("login"), got.Get("api_key"), tt.wantLogin, tt.wantKey)
			}
		})
	}
}

func TestMalformedBody(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	})

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	var bodyErr *fetch.BodyError
	if !errors.As(err, &bodyErr) {
		t.Fatalf("error = %v, want a typed *fetch.BodyError", err)
	}

	if !strings.Contains(err.Error(), "danbooru") || !strings.Contains(err.Error(), "/posts.json") ||
		!strings.Contains(err.Error(), "non-JSON") {
		t.Errorf("error = %q, want it to name the client, path, and non-JSON body", err)
	}
}

func TestRelatedTagsDecodesTagObjectsAndFloatFrequency(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"query":"cat_ears","related_tags":[
			{"tag":{"id":6126,"name":"animal_ears","post_count":1765194},"frequency":1.0},
			{"tag":{"id":470575,"name":"1girl"},"frequency":0.7134}
		]}`))
	})

	tags, err := client.RelatedTags(context.Background(), booru.RelatedQuery{Tag: "cat_ears"})
	if err != nil {
		t.Fatalf("RelatedTags() error: %v", err)
	}

	if len(tags) != 2 || tags[0].Tag != "animal_ears" || tags[0].Score != 1.0 || tags[1].Score != 0.7134 {
		t.Errorf("tags = %+v, want names and float scores from the tag objects", tags)
	}

	if tags[0].Rank != 1 || tags[1].Rank != 2 {
		t.Errorf("ranks = %d, %d, want 1 then 2", tags[0].Rank, tags[1].Rank)
	}
}
