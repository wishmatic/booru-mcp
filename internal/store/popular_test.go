package store

import (
	"context"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestReplaceAndReadPopular(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	tags := []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 900},
		{Name: "red_hair", Category: booru.CategoryGeneral, Count: 500},
	}

	if err := client.ReplacePopular(ctx, "danbooru", tags, at); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	got, err := client.Popular(ctx, "danbooru")
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if len(got) != 2 || got[0].Tag.Name != "blue_eyes" || got[1].Tag.Name != "red_hair" {
		t.Fatalf("Popular() = %v, want rank order preserved", names(got))
	}

	if !got[0].FetchedAt.Equal(at) {
		t.Errorf("FetchedAt = %v, want %v", got[0].FetchedAt, at)
	}

	if !got[0].IsPopular {
		t.Error("IsPopular = false, want true for a popular row")
	}

	if popular, err := client.IsPopular(ctx, "danbooru", "blue_eyes"); err != nil || !popular {
		t.Errorf("IsPopular(blue_eyes) = %v, %v, want true", popular, err)
	}
}

func TestReplacePopularIsScopedToClient(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	if err := client.ReplacePopular(ctx, "danbooru", []booru.Tag{{Name: "blue_eyes", Count: 900}}, at); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	if err := client.ReplacePopular(ctx, "rule34", []booru.Tag{{Name: "red_hair", Count: 400}}, at); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	if err := client.ReplacePopular(ctx, "danbooru", []booru.Tag{{Name: "green_eyes", Count: 800}}, at); err != nil {
		t.Fatalf("ReplacePopular() error: %v", err)
	}

	danbooru, err := client.Popular(ctx, "danbooru")
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if len(danbooru) != 1 || danbooru[0].Tag.Name != "green_eyes" {
		t.Errorf("Popular(danbooru) = %v, want only the replaced set", names(danbooru))
	}

	rule34, err := client.Popular(ctx, "rule34")
	if err != nil {
		t.Fatalf("Popular() error: %v", err)
	}

	if len(rule34) != 1 || rule34[0].Tag.Name != "red_hair" {
		t.Errorf("Popular(rule34) = %v, want it untouched", names(rule34))
	}

	if popular, err := client.IsPopular(ctx, "danbooru", "blue_eyes"); err != nil || popular {
		t.Errorf("IsPopular(blue_eyes) = %v, %v, want false after replacement", popular, err)
	}
}
