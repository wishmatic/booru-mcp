package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func tag(name string, count int) booru.Tag {
	return booru.Tag{Name: name, Category: booru.CategoryGeneral, Count: count}
}

func TestTagsFreshCacheServesWithoutProvider(t *testing.T) {
	provider := &fakeProvider{name: "danbooru"}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	if err := service.store.UpsertTags(context.Background(), "danbooru", []booru.Tag{tag("blue_eyes", 100)}, testTime); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	result, err := service.Tags(context.Background(), TagsInput{Query: "blue"})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 0 {
		t.Errorf("tagCalls = %d, want 0 for a fresh cache hit", provider.tagCalls)
	}

	if len(result.Tags) != 1 || result.Tags[0].Score != 100 {
		t.Errorf("Tags = %+v, want the cached tag at 100%%", result.Tags)
	}
}

func TestTagsMissFetchesStoresAndScores(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 100), tag("blue_sky", 50)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Tags(context.Background(), TagsInput{Query: "blue"})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 1 {
		t.Fatalf("tagCalls = %d, want 1 on a miss", provider.tagCalls)
	}

	if len(result.Tags) != 2 || result.Tags[0].Name != "blue_eyes" || result.Tags[0].Score != 100 ||
		result.Tags[1].Score != 50 {
		t.Errorf("Tags = %+v, want descending percentages from the query top", result.Tags)
	}

	stored, err := service.store.SearchTags(context.Background(), storeFilter("danbooru", "blue"))
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(stored) != 2 {
		t.Errorf("stored = %d tags, want 2", len(stored))
	}
}

func TestTagsStalePopularRefetches(t *testing.T) {
	stale := testTime.Add(-2 * time.Hour)

	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 999)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	seed := []booru.Tag{tag("blue_eyes", 100)}

	if err := service.store.UpsertTags(context.Background(), "danbooru", seed, stale); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	if err := service.store.ReplacePopular(context.Background(), "danbooru", seed, stale); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	result, err := service.Tags(context.Background(), TagsInput{Query: "blue"})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 1 {
		t.Fatalf("tagCalls = %d, want 1 because a stale popular entry was re-fetched", provider.tagCalls)
	}

	if result.Tags[0].Count != 999 {
		t.Errorf("count = %d, want the refreshed value", result.Tags[0].Count)
	}
}

func TestTagsStaleNonPopularServedStale(t *testing.T) {
	stale := testTime.Add(-2 * time.Hour)

	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 999)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	if err := service.store.UpsertTags(context.Background(), "danbooru", []booru.Tag{tag("blue_eyes", 100)}, stale); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	result, err := service.Tags(context.Background(), TagsInput{Query: "blue"})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 0 {
		t.Errorf("tagCalls = %d, want 0 for a stale non-popular entry", provider.tagCalls)
	}

	if result.Tags[0].Count != 100 {
		t.Errorf("count = %d, want the stale value", result.Tags[0].Count)
	}
}

func TestTagsRefreshForcesFetch(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 999)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	if err := service.store.UpsertTags(context.Background(), "danbooru", []booru.Tag{tag("blue_eyes", 100)}, testTime); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	if _, err := service.Tags(context.Background(), TagsInput{Query: "blue", Refresh: true}); err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 1 {
		t.Errorf("tagCalls = %d, want 1 when refresh is set", provider.tagCalls)
	}
}

func TestTagsCacheDisabled(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 100)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: 0},
		testEntry{name: "danbooru", provider: provider})

	if _, err := service.Tags(context.Background(), TagsInput{Query: "blue"}); err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if provider.tagCalls != 1 {
		t.Errorf("tagCalls = %d, want 1 with caching disabled", provider.tagCalls)
	}

	stored, err := service.store.SearchTags(context.Background(), storeFilter("danbooru", ""))
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(stored) != 0 {
		t.Errorf("stored = %d tags, want nothing written with caching disabled", len(stored))
	}
}

func TestTagsDeduplicatesByPercentage(t *testing.T) {
	small := &fakeProvider{name: "small", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 90), tag("filler", 100)}, nil
	}}
	large := &fakeProvider{name: "large", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 60_000), tag("filler", 100_000)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour, DefaultClients: []string{"small", "large"}},
		testEntry{name: "small", provider: small},
		testEntry{name: "large", provider: large},
	)

	result, err := service.Tags(context.Background(), TagsInput{Query: ""})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	var blue *booru.FusedTag

	for i := range result.Tags {
		if result.Tags[i].Name == "blue_eyes" {
			blue = &result.Tags[i]
		}
	}

	if blue == nil {
		t.Fatal("blue_eyes missing from the result")
	}

	if blue.Client != "small" || blue.Score != 90 || blue.Count != 90 {
		t.Errorf("blue_eyes = %+v, want the small client's 90%% instance despite its smaller raw count", blue)
	}

	if len(blue.Clients) != 2 {
		t.Errorf("per-client breakdown = %+v, want both clients", blue.Clients)
	}
}

func TestTagsCapsLimit(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("a", 5), tag("b", 4), tag("c", 3)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 2, CacheTTL: 0},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Tags(context.Background(), TagsInput{Query: "", Limit: 2})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if len(result.Tags) != 2 {
		t.Errorf("Tags = %d, want 2", len(result.Tags))
	}
}
