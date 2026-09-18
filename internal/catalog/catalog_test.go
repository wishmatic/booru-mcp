package catalog

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/store"
)

var testTime = time.Unix(1_700_000_000, 0).UTC()

type fakeProvider struct {
	name string
	caps booru.Capabilities

	searchFn  func(booru.SearchParams) ([]booru.Post, error)
	postFn    func(string) (booru.Post, error)
	tagsFn    func(booru.TagQuery) ([]booru.Tag, error)
	popularFn func(booru.PopularQuery) ([]booru.Tag, error)

	searchParams booru.SearchParams
	searchCalls  int
	tagCalls     int
	popularCalls int
}

func (f *fakeProvider) Name() string {
	return f.name
}

func (f *fakeProvider) Capabilities() booru.Capabilities {
	return f.caps
}

func (f *fakeProvider) Search(_ context.Context, params booru.SearchParams) ([]booru.Post, error) {
	f.searchCalls++
	f.searchParams = params

	if f.searchFn != nil {
		return f.searchFn(params)
	}

	return nil, nil
}

func (f *fakeProvider) Post(_ context.Context, id string) (booru.Post, error) {
	if f.postFn != nil {
		return f.postFn(id)
	}

	return booru.Post{Client: f.name, ID: id, URL: "https://example.com/" + id}, nil
}

func (f *fakeProvider) SearchTags(_ context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	f.tagCalls++

	if f.tagsFn != nil {
		return f.tagsFn(query)
	}

	return nil, nil
}

func (f *fakeProvider) PopularTags(_ context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	f.popularCalls++

	if f.popularFn != nil {
		return f.popularFn(query)
	}

	return nil, nil
}

type relatedProvider struct {
	*fakeProvider
	relatedFn    func(booru.RelatedQuery) ([]booru.RelatedTag, error)
	relatedCalls int
}

func (f *relatedProvider) RelatedTags(_ context.Context, query booru.RelatedQuery) ([]booru.RelatedTag, error) {
	f.relatedCalls++

	var tags []booru.RelatedTag

	if f.relatedFn != nil {
		var err error

		tags, err = f.relatedFn(query)
		if err != nil {
			return nil, err
		}
	}

	for i := range tags {
		if tags[i].Client == "" {
			tags[i].Client = f.name
		}
	}

	return tags, nil
}

type testEntry struct {
	name     string
	provider booru.Provider
	reason   string
}

func newTestService(t *testing.T, opts Options, entries ...testEntry) *Service {
	t.Helper()

	registry := booru.NewRegistry()

	for _, entry := range entries {
		if err := registry.Register(entry.name, entry.provider, entry.reason); err != nil {
			t.Fatalf("Register(%s) error: %v", entry.name, err)
		}
	}

	db, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if len(opts.DefaultClients) == 0 && len(entries) > 0 {
		opts.DefaultClients = []string{entries[0].name}
	}

	service := New(registry, db, opts)
	service.now = func() time.Time { return testTime }

	return service
}

func post(client, id string, rating booru.Rating, tags ...string) booru.Post {
	postTags := make([]booru.Tag, 0, len(tags))
	for _, tag := range tags {
		postTags = append(postTags, booru.Tag{Name: tag, Category: booru.CategoryGeneral})
	}

	return booru.Post{Client: client, ID: id, URL: "https://example.com/" + id, Rating: rating, Tags: postTags}
}

func storeFilter(client, query string) store.TagFilter {
	return store.TagFilter{Client: client, Query: query}
}

func TestSearchRejectsUnknownClient(t *testing.T) {
	service := newTestService(t, Options{MaxLimit: 10}, testEntry{name: "danbooru", provider: &fakeProvider{name: "danbooru"}})

	if _, err := service.Search(context.Background(), SearchInput{Tags: "x", Clients: []string{"nope"}}); err == nil {
		t.Fatal("Search() error = nil, want an error for an unknown client")
	}
}

func TestSearchQueriesDefaultClients(t *testing.T) {
	first := &fakeProvider{name: "danbooru"}
	second := &fakeProvider{name: "rule34"}

	service := newTestService(t,
		Options{MaxLimit: 10, DefaultClients: []string{"danbooru", "rule34"}},
		testEntry{name: "danbooru", provider: first},
		testEntry{name: "rule34", provider: second},
	)

	if _, err := service.Search(context.Background(), SearchInput{Tags: "x"}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if first.searchCalls != 1 || second.searchCalls != 1 {
		t.Errorf("calls = %d, %d, want one each", first.searchCalls, second.searchCalls)
	}
}

func TestSearchSkipsInactiveClients(t *testing.T) {
	service := newTestService(t,
		Options{MaxLimit: 10, DefaultClients: []string{"danbooru", "rule34"}},
		testEntry{name: "danbooru", provider: &fakeProvider{name: "danbooru"}},
		testEntry{name: "rule34", reason: "RULE34_API_KEY is not set"},
	)

	result, err := service.Search(context.Background(), SearchInput{Tags: "x"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(result.Skipped) != 1 || result.Skipped[0].Client != "rule34" || result.Skipped[0].Reason == "" {
		t.Errorf("Skipped = %+v, want rule34 with a reason", result.Skipped)
	}

	if len(result.Posts) != 0 {
		t.Errorf("Posts = %+v, want none", result.Posts)
	}
}

func TestSearchCapsLimitAndNarrowsRating(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", caps: booru.Capabilities{Rating: true}}

	service := newTestService(t, Options{MaxLimit: 5, ContentRating: booru.RatingGeneral},
		testEntry{name: "danbooru", provider: provider})

	if _, err := service.Search(context.Background(), SearchInput{Tags: "x", Limit: 50, Rating: booru.RatingExplicit}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if provider.searchParams.Limit != 5 {
		t.Errorf("limit = %d, want it capped at 5", provider.searchParams.Limit)
	}

	if provider.searchParams.Rating != booru.RatingGeneral {
		t.Errorf("rating = %q, want it narrowed to general", provider.searchParams.Rating)
	}
}

func TestSearchExcludesRatinglessClient(t *testing.T) {
	provider := &fakeProvider{name: "unrated"}

	service := newTestService(t, Options{MaxLimit: 10, ContentRating: booru.RatingGeneral},
		testEntry{name: "unrated", provider: provider})

	result, err := service.Search(context.Background(), SearchInput{Tags: "x"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if provider.searchCalls != 0 {
		t.Errorf("searchCalls = %d, want 0 for a client that cannot rate-filter", provider.searchCalls)
	}

	if len(result.Skipped) != 1 || result.Skipped[0].Client != "unrated" {
		t.Errorf("Skipped = %+v, want unrated excluded", result.Skipped)
	}
}

func TestSearchAppliesBlockedTags(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return []booru.Post{
			post("danbooru", "1", booru.RatingGeneral, "bad_tag"),
			post("danbooru", "2", booru.RatingGeneral, "fine"),
		}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10, BlockedTags: []string{"bad_tag"}},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Search(context.Background(), SearchInput{Tags: "x"})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(result.Posts) != 1 || result.Posts[0].ID != "2" {
		t.Errorf("Posts = %+v, want only the unblocked post", result.Posts)
	}

	if len(provider.searchParams.Exclude) != 1 || provider.searchParams.Exclude[0] != "bad_tag" {
		t.Errorf("Exclude = %v, want the blocked tag negated upstream", provider.searchParams.Exclude)
	}
}

func TestSearchOrdersByRequestedClientOrder(t *testing.T) {
	first := &fakeProvider{name: "danbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return []booru.Post{post("danbooru", "1", booru.RatingGeneral)}, nil
	}}
	second := &fakeProvider{name: "rule34", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return []booru.Post{post("rule34", "2", booru.RatingGeneral)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10},
		testEntry{name: "danbooru", provider: first},
		testEntry{name: "rule34", provider: second},
	)

	result, err := service.Search(context.Background(), SearchInput{Tags: "x", Clients: []string{"rule34", "danbooru"}})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(result.Posts) != 2 || result.Posts[0].Client != "rule34" || result.Posts[1].Client != "danbooru" {
		t.Errorf("Posts order = %+v, want rule34 first", result.Posts)
	}
}

func TestSearchPartialAndTotalFailure(t *testing.T) {
	failing := &fakeProvider{name: "rule34", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return nil, errors.New("boom")
	}}
	healthy := &fakeProvider{name: "danbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return []booru.Post{post("danbooru", "1", booru.RatingGeneral)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10},
		testEntry{name: "danbooru", provider: healthy},
		testEntry{name: "rule34", provider: failing},
	)

	result, err := service.Search(context.Background(), SearchInput{Tags: "x", Clients: []string{"danbooru", "rule34"}})
	if err != nil {
		t.Fatalf("Search() error: %v, want a partial failure to succeed", err)
	}

	if len(result.Warnings) != 1 || len(result.Posts) != 1 {
		t.Errorf("result = %+v, want one warning and one post", result)
	}

	if _, err := service.Search(context.Background(), SearchInput{Tags: "x", Clients: []string{"rule34"}}); err == nil {
		t.Fatal("Search() error = nil, want an error when every client fails")
	}
}

func TestGetDefaultsToDanbooru(t *testing.T) {
	danbooru := &fakeProvider{name: "danbooru"}
	rule34 := &fakeProvider{name: "rule34"}

	service := newTestService(t, Options{MaxLimit: 10, DefaultClients: []string{"rule34", "danbooru"}},
		testEntry{name: "danbooru", provider: danbooru},
		testEntry{name: "rule34", provider: rule34},
	)

	got, err := service.Get(context.Background(), "", "5")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got.Client != "danbooru" {
		t.Errorf("client = %q, want danbooru", got.Client)
	}
}

func TestGetErrors(t *testing.T) {
	service := newTestService(t,
		Options{MaxLimit: 10, ContentRating: booru.RatingGeneral, BlockedTags: []string{"bad_tag"}},
		testEntry{name: "danbooru", provider: &fakeProvider{name: "danbooru", postFn: func(id string) (booru.Post, error) {
			return post("danbooru", id, booru.RatingExplicit, "bad_tag"), nil
		}}},
		testEntry{name: "rule34", reason: "RULE34_API_KEY is not set"},
	)

	if _, err := service.Get(context.Background(), "nope", "1"); err == nil {
		t.Error("Get() with an unknown client error = nil, want an error")
	}

	if _, err := service.Get(context.Background(), "rule34", "1"); err == nil {
		t.Error("Get() with an inactive client error = nil, want an error")
	}

	if _, err := service.Get(context.Background(), "danbooru", "1"); err == nil {
		t.Error("Get() above the rating cap and with a blocked tag error = nil, want an error")
	}
}

func TestGetNotFound(t *testing.T) {
	service := newTestService(t, Options{MaxLimit: 10},
		testEntry{name: "danbooru", provider: &fakeProvider{name: "danbooru", postFn: func(string) (booru.Post, error) {
			return booru.Post{}, errors.New("danbooru: post 9 not found")
		}}},
	)

	if _, err := service.Get(context.Background(), "danbooru", "9"); err == nil {
		t.Fatal("Get() error = nil, want a not-found error")
	}
}
