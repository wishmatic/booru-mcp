package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func relatedTag(name string, score float64) booru.RelatedTag {
	return booru.RelatedTag{Tag: name, Score: score}
}

func TestRelatedSkipsClientsWithoutTheAPI(t *testing.T) {
	plain := &fakeProvider{name: "yandere"}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: time.Hour, DefaultClients: []string{"yandere"}},
		testEntry{name: "yandere", provider: plain})

	result, err := service.Related(context.Background(), RelatedInput{Tag: "smile"})
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if len(result.Tags) != 0 {
		t.Errorf("Tags = %+v, want none", result.Tags)
	}

	if len(result.Skipped) != 1 || result.Skipped[0].Client != "yandere" || result.Skipped[0].Reason == "" {
		t.Errorf("Skipped = %+v, want yandere with a reason", result.Skipped)
	}
}

func TestRelatedFreshCacheServesWithoutProvider(t *testing.T) {
	provider := &relatedProvider{fakeProvider: &fakeProvider{name: "danbooru"}}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	if err := service.store.ReplaceRelated(context.Background(), "danbooru", "smile",
		[]booru.RelatedTag{relatedTag("blue_eyes", 900)}, testTime); err != nil {
		t.Fatalf("ReplaceRelated() error: %v", err)
	}

	result, err := service.Related(context.Background(), RelatedInput{Tag: "smile"})
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if provider.relatedCalls != 0 {
		t.Errorf("relatedCalls = %d, want 0 for a fresh cache hit", provider.relatedCalls)
	}

	if len(result.Tags) != 1 || result.Tags[0].Tag != "blue_eyes" {
		t.Errorf("Tags = %+v, want the cached related tag", result.Tags)
	}
}

func TestRelatedFetchesAndStores(t *testing.T) {
	provider := &relatedProvider{
		fakeProvider: &fakeProvider{name: "danbooru"},
		relatedFn: func(booru.RelatedQuery) ([]booru.RelatedTag, error) {
			return []booru.RelatedTag{relatedTag("blue_eyes", 900)}, nil
		},
	}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Related(context.Background(), RelatedInput{Tag: "Smile"})
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if provider.relatedCalls != 1 || len(result.Tags) != 1 {
		t.Fatalf("result = %+v, calls = %d", result, provider.relatedCalls)
	}

	stored, err := service.store.Related(context.Background(), "danbooru", "smile")
	if err != nil || len(stored) != 1 {
		t.Fatalf("stored = %+v, %v, want the fetched tag under the normalized name", stored, err)
	}
}

func TestRelatedKeepsClientsSeparate(t *testing.T) {
	first := &relatedProvider{
		fakeProvider: &fakeProvider{name: "danbooru"},
		relatedFn: func(booru.RelatedQuery) ([]booru.RelatedTag, error) {
			return []booru.RelatedTag{relatedTag("blue_eyes", 900)}, nil
		},
	}
	second := &relatedProvider{
		fakeProvider: &fakeProvider{name: "gelbooru"},
		relatedFn: func(booru.RelatedQuery) ([]booru.RelatedTag, error) {
			return []booru.RelatedTag{relatedTag("blue_eyes", 800)}, nil
		},
	}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: 0, DefaultClients: []string{"danbooru", "gelbooru"}},
		testEntry{name: "danbooru", provider: first},
		testEntry{name: "gelbooru", provider: second},
	)

	result, err := service.Related(context.Background(), RelatedInput{Tag: "smile"})
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if len(result.Tags) != 2 {
		t.Fatalf("Tags = %+v, want both clients' rows kept separately", result.Tags)
	}

	if result.Tags[0].Client != "danbooru" || result.Tags[1].Client != "gelbooru" {
		t.Errorf("clients = %q, %q, want danbooru then gelbooru", result.Tags[0].Client, result.Tags[1].Client)
	}
}

func TestRelatedCapsLimit(t *testing.T) {
	provider := &relatedProvider{
		fakeProvider: &fakeProvider{name: "danbooru"},
		relatedFn: func(booru.RelatedQuery) ([]booru.RelatedTag, error) {
			return []booru.RelatedTag{relatedTag("a", 3), relatedTag("b", 2), relatedTag("c", 1)}, nil
		},
	}

	service := newTestService(t, Options{MaxLimit: 2, CacheTTL: 0},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Related(context.Background(), RelatedInput{Tag: "smile", Limit: 2})
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if len(result.Tags) != 2 {
		t.Errorf("Tags = %d, want 2", len(result.Tags))
	}
}
