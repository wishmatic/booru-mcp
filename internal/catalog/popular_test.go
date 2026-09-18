package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestPopularFreshSnapshotServesWithoutProvider(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
		return nil, errors.New("provider must not be called")
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	if err := service.store.ReplacePopular(context.Background(), "danbooru", []booru.Tag{tag("blue_eyes", 100)}, testTime); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	result, err := service.Popular(context.Background(), PopularInput{})
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if provider.popularCalls != 0 {
		t.Errorf("popularCalls = %d, want 0 for a fresh snapshot", provider.popularCalls)
	}

	if len(result.Tags) != 1 || result.Tags[0].Name != "blue_eyes" {
		t.Errorf("Tags = %+v, want the cached snapshot", result.Tags)
	}
}

func TestPopularStaleRefetches(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 900), tag("smile", 500)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: time.Hour},
		testEntry{name: "danbooru", provider: provider})

	stale := testTime.Add(-2 * time.Hour)
	if err := service.store.ReplacePopular(context.Background(), "danbooru", []booru.Tag{tag("blue_eyes", 1)}, stale); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	result, err := service.Popular(context.Background(), PopularInput{})
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if provider.popularCalls != 1 {
		t.Fatalf("popularCalls = %d, want 1 for a stale snapshot", provider.popularCalls)
	}

	if len(result.Tags) != 2 {
		t.Errorf("Tags = %+v, want the refreshed snapshot", result.Tags)
	}
}

func TestPopularSkipsInactiveClients(t *testing.T) {
	service := newTestService(t,
		Options{MaxLimit: 100, CacheTTL: time.Hour, DefaultClients: []string{"danbooru", "rule34"}},
		testEntry{name: "danbooru", provider: &fakeProvider{name: "danbooru", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
			return []booru.Tag{tag("blue_eyes", 100)}, nil
		}}},
		testEntry{name: "rule34", reason: "RULE34_API_KEY is not set"},
	)

	result, err := service.Popular(context.Background(), PopularInput{})
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if len(result.Skipped) != 1 || result.Skipped[0].Client != "rule34" {
		t.Errorf("Skipped = %+v, want rule34", result.Skipped)
	}

	if len(result.Tags) != 1 {
		t.Errorf("Tags = %+v, want only the active client's snapshot", result.Tags)
	}
}

func TestPopularPartialAndTotalFailure(t *testing.T) {
	failing := &fakeProvider{name: "rule34", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
		return nil, errors.New("boom")
	}}
	healthy := &fakeProvider{name: "danbooru", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 100)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 100, CacheTTL: 0},
		testEntry{name: "danbooru", provider: healthy},
		testEntry{name: "rule34", provider: failing},
	)

	result, err := service.Popular(context.Background(), PopularInput{Clients: []string{"danbooru", "rule34"}})
	if err != nil {
		t.Fatalf("Popular() error: %v, want a partial failure to succeed", err)
	}

	if len(result.Warnings) != 1 || len(result.Tags) != 1 {
		t.Errorf("result = %+v, want one warning and one tag", result)
	}

	if _, err := service.Popular(context.Background(), PopularInput{Clients: []string{"rule34"}}); err == nil {
		t.Fatal("Popular() error = nil, want an error when every client fails")
	}
}

func TestPopularCapsLimit(t *testing.T) {
	provider := &fakeProvider{name: "danbooru", popularFn: func(booru.PopularQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("a", 5), tag("b", 4), tag("c", 3)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 2, CacheTTL: 0},
		testEntry{name: "danbooru", provider: provider})

	result, err := service.Popular(context.Background(), PopularInput{Limit: 2})
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if len(result.Tags) != 2 {
		t.Errorf("Tags = %d, want 2", len(result.Tags))
	}
}

func TestFuseRanksAcrossClients(t *testing.T) {
	perClient := map[string][]booru.Tag{
		"a": {tag("x", 100), tag("a1", 90), tag("a2", 80)},
		"b": {tag("b1", 100), tag("b2", 90), tag("x", 80)},
		"c": {tag("y", 100), tag("c1", 90), tag("c2", 80)},
	}

	fused := Fuse(perClient)
	if len(fused) == 0 {
		t.Fatal("Fuse() returned nothing")
	}

	if fused[0].Name != "x" {
		t.Errorf("top tag = %q, want x: strong on two clients beats strong on one", fused[0].Name)
	}

	var x *booru.FusedTag

	for i := range fused {
		if fused[i].Name == "x" {
			x = &fused[i]
		}
	}

	if x == nil || len(x.Clients) != 2 {
		t.Fatalf("x = %+v, want a per-client breakdown for both clients", x)
	}
}
