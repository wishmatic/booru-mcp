package store

import (
	"context"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestUpsertAndReadTags(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	tags := []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 100},
		{Name: "somebody", Category: booru.CategoryArtist, Count: 50},
	}

	if err := client.UpsertTags(ctx, "danbooru", tags, at); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	got, err := client.TagsByClient(ctx, "danbooru", []string{"blue_eyes"})
	if err != nil {
		t.Fatalf("TagsByClient() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("TagsByClient() = %d rows, want 1", len(got))
	}

	if got[0].Tag.Category != booru.CategoryGeneral || got[0].Tag.Count != 100 {
		t.Errorf("tag = %+v, want general/100", got[0].Tag)
	}

	if !got[0].FetchedAt.Equal(at) {
		t.Errorf("FetchedAt = %v, want %v", got[0].FetchedAt, at)
	}

	if got[0].IsPopular {
		t.Error("IsPopular = true, want false")
	}
}

func TestUpsertReplaces(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()

	first := time.Unix(1_700_000_000, 0).UTC()
	second := first.Add(time.Hour)

	if err := client.UpsertTags(ctx, "danbooru", []booru.Tag{{Name: "blue_eyes", Count: 100}}, first); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	if err := client.UpsertTags(ctx, "danbooru", []booru.Tag{{Name: "blue_eyes", Count: 200}}, second); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	got, err := client.SearchTags(ctx, TagFilter{Client: "danbooru"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("SearchTags() = %d rows, want 1 after an upsert", len(got))
	}

	if got[0].Tag.Count != 200 || !got[0].FetchedAt.Equal(second) {
		t.Errorf("row = %+v, want the replaced count and timestamp", got[0])
	}
}

func TestSearchTagsOrderingAndFilters(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	tags := []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 100},
		{Name: "blue_sky", Category: booru.CategoryGeneral, Count: 10},
		{Name: "blue_artist", Category: booru.CategoryArtist, Count: 50},
		{Name: "red_hair", Category: booru.CategoryGeneral, Count: 500},
	}

	if err := client.UpsertTags(ctx, "danbooru", tags, at); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	all, err := client.SearchTags(ctx, TagFilter{Client: "danbooru", Query: "blue"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("SearchTags(blue) = %d rows, want 3", len(all))
	}

	if all[0].Tag.Name != "blue_eyes" || all[1].Tag.Name != "blue_artist" || all[2].Tag.Name != "blue_sky" {
		t.Errorf("SearchTags(blue) order = %v, want count descending", names(all))
	}

	limited, err := client.SearchTags(ctx, TagFilter{Client: "danbooru", Query: "blue", Limit: 1})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(limited) != 1 || limited[0].Tag.Name != "blue_eyes" {
		t.Errorf("SearchTags(blue, limit 1) = %v, want just blue_eyes", names(limited))
	}

	artists, err := client.SearchTags(ctx, TagFilter{Client: "danbooru", Query: "blue", Category: booru.CategoryArtist})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(artists) != 1 || artists[0].Tag.Name != "blue_artist" {
		t.Errorf("SearchTags(blue, artist) = %v, want just blue_artist", names(artists))
	}
}

func TestSearchTagsEscapesWildcards(t *testing.T) {
	client := newTestStore(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 0).UTC()

	tags := []booru.Tag{
		{Name: "100%_cute", Count: 5},
		{Name: "100x_cute", Count: 5},
	}

	if err := client.UpsertTags(ctx, "danbooru", tags, at); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	got, err := client.SearchTags(ctx, TagFilter{Client: "danbooru", Query: "0%_"})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(got) != 1 || got[0].Tag.Name != "100%_cute" {
		t.Errorf("SearchTags(0%%_) = %v, want only 100%%_cute", names(got))
	}
}

func names(tags []CachedTag) []string {
	out := make([]string, 0, len(tags))

	for _, tag := range tags {
		out = append(out, tag.Tag.Name)
	}

	return out
}
