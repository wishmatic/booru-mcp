package store

import (
	"context"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestReplaceAndReadRelated(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	tags := []booru.RelatedTag{
		{Tag: "blue_eyes", Score: 900},
		{Tag: "red_hair", Score: 400},
	}

	if err := client.ReplaceRelated(ctx, "danbooru", "smile", tags, at); err != nil {
		t.Fatalf("ReplaceRelated() error: %v", err)
	}

	got, err := client.Related(ctx, "danbooru", "smile")
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if len(got) != 2 || got[0].Tag != "blue_eyes" || got[0].Score != 900 || got[0].Rank != 1 {
		t.Fatalf("Related() = %+v, want score and rank round-tripped", got)
	}

	if got[1].Rank != 2 {
		t.Errorf("rank = %d, want 2", got[1].Rank)
	}

	fetchedAt, ok, err := client.RelatedFetchedAt(ctx, "danbooru", "smile")
	if err != nil {
		t.Fatalf("RelatedFetchedAt() error: %v", err)
	}

	if !ok || !fetchedAt.Equal(at) {
		t.Errorf("RelatedFetchedAt() = %v, %v, want %v, true", fetchedAt, ok, at)
	}
}

func TestReplaceRelatedIsScoped(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	if err := client.ReplaceRelated(ctx, "danbooru", "smile", []booru.RelatedTag{{Tag: "blue_eyes"}}, at); err != nil {
		t.Fatalf("ReplaceRelated() error: %v", err)
	}

	if err := client.ReplaceRelated(ctx, "danbooru", "cry", []booru.RelatedTag{{Tag: "tears"}}, at); err != nil {
		t.Fatalf("ReplaceRelated() error: %v", err)
	}

	smile, err := client.Related(ctx, "danbooru", "smile")
	if err != nil {
		t.Fatalf("Related() error: %v", err)
	}

	if len(smile) != 1 || smile[0].Tag != "blue_eyes" {
		t.Errorf("Related(smile) = %+v, want it untouched", smile)
	}

	if _, ok, err := client.RelatedFetchedAt(ctx, "danbooru", "missing"); err != nil || ok {
		t.Errorf("RelatedFetchedAt(missing) = %v, %v, want false", ok, err)
	}
}
