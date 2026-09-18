package gelbooru

import (
	"context"
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

func newTestProvider(t *testing.T, handler http.HandlerFunc, name, apiKey, userID string) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	return New(Config{
		Name:           name,
		BaseURL:        server.URL,
		APIKey:         apiKey,
		UserID:         userID,
		MaxLimit:       3,
		CredentialEnvs: []string{strings.ToUpper(name) + "_API_KEY", strings.ToUpper(name) + "_USER_ID"},
		HTTP:           httpClient,
	})
}

func TestSearchRequest(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"post":[]}`))
	}, "gelbooru", "key", "user")

	_, err := client.Search(context.Background(), booru.SearchParams{
		Tags:    "blue eyes",
		Limit:   10,
		Page:    3,
		Rating:  booru.RatingExplicit,
		Exclude: []string{"Bad Tag"},
	})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	for key, want := range map[string]string{
		"page": "dapi", "s": "post", "q": "index", "json": "1",
		"limit": "3", "pid": "2", "api_key": "key", "user_id": "user",
	} {
		if got.Get(key) != want {
			t.Errorf("%s = %q, want %q", key, got.Get(key), want)
		}
	}

	if got.Get("tags") != "blue eyes rating:explicit -bad_tag" {
		t.Errorf("tags = %q", got.Get("tags"))
	}
}

func TestKeylessSendsNoCredentials(t *testing.T) {
	var got url.Values

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_, _ = w.Write([]byte(`{"post":[]}`))
	}, "safebooru", "", "")

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if got.Get("api_key") != "" || got.Get("user_id") != "" {
		t.Errorf("api_key=%q user_id=%q, want neither", got.Get("api_key"), got.Get("user_id"))
	}
}

func TestTagsMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag":[
			{"name":"Blue Eyes","count":900,"type":0},
			{"name":"somebody","count":500,"type":1},
			{"name":"deprecated_thing","count":10,"type":6},
			{"name":"from_category_key","count":7,"category":4}
		]}`))
	}, "gelbooru", "", "")

	tags, err := client.SearchTags(context.Background(), booru.TagQuery{Query: "blue"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := map[string]booru.TagCategory{
		"blue_eyes":         booru.CategoryGeneral,
		"somebody":          booru.CategoryArtist,
		"deprecated_thing":  booru.CategoryGeneral,
		"from_category_key": booru.CategoryCharacter,
	}

	for _, tag := range tags {
		if want[tag.Name] != tag.Category {
			t.Errorf("tag %s category = %q, want %q", tag.Name, tag.Category, want[tag.Name])
		}
	}

	if len(tags) != len(want) {
		t.Fatalf("tags = %d, want %d", len(tags), len(want))
	}
}

func TestPostMapping(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"post":[{
			"id": 77,
			"created_at": "2024-01-02 03:04:05",
			"score": 4,
			"width": 100,
			"height": 200,
			"rating": "questionable",
			"source": "https://example.com",
			"file_url": "https://cdn.example.com/77.png",
			"sample_url": "https://cdn.example.com/77-sample.png",
			"preview_url": "https://cdn.example.com/77-preview.png",
			"tags": "blue eyes smile"
		}]}`))
	}, "gelbooru", "", "")

	post, err := client.Post(context.Background(), "77")
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}

	if post.URL != client.baseURL+"/index.php?page=post&s=view&id=77" {
		t.Errorf("URL = %q, want the Gelbooru permalink", post.URL)
	}

	if post.Rating != booru.RatingQuestionable || post.Score != 4 || post.Width != 100 {
		t.Errorf("post = %+v", post)
	}

	if len(post.Tags) != 3 || post.Tags[0].Name != "blue" || post.Tags[0].Category != booru.CategoryGeneral {
		t.Errorf("tags = %+v, want a space-split general tag list", post.Tags)
	}

	if post.CreatedAt.IsZero() {
		t.Error("CreatedAt = zero, want the parsed timestamp")
	}
}

func TestUnauthorizedNamesCredentials(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}, "rule34", "super-secret-key", "user")

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "x"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	for _, want := range []string{"rule34", "RULE34_API_KEY", "RULE34_USER_ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}

	if strings.Contains(err.Error(), "super-secret-key") {
		t.Errorf("error %q leaks the API key", err.Error())
	}
}
