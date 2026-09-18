package booru

import (
	"context"
	"testing"
)

type stubProvider struct{}

func (stubProvider) Name() string               { return "stub" }
func (stubProvider) Capabilities() Capabilities { return Capabilities{Rating: true, Random: false} }

func (stubProvider) Search(context.Context, SearchParams) ([]Post, error) { return nil, nil }
func (stubProvider) Post(context.Context, string) (Post, error)           { return Post{}, nil }
func (stubProvider) SearchTags(context.Context, TagQuery) ([]Tag, error)  { return nil, nil }
func (stubProvider) PopularTags(context.Context, PopularQuery) ([]Tag, error) {
	return nil, nil
}

var _ Provider = stubProvider{}

func TestNormalizeTag(t *testing.T) {
	tests := map[string]string{
		"Blue Eyes":     "blue_eyes",
		"  BLUE  EYES ": "blue_eyes",
		"blue_eyes":     "blue_eyes",
		"":              "",
		"One":           "one",
	}

	for in, want := range tests {
		if got := NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTagCategory(t *testing.T) {
	if got, err := ParseTagCategory("Artist"); err != nil || got != CategoryArtist {
		t.Fatalf("ParseTagCategory(Artist) = %q, %v", got, err)
	}

	if _, err := ParseTagCategory("nonsense"); err == nil {
		t.Error("ParseTagCategory(nonsense) error = nil, want an error")
	}
}

func TestParseRating(t *testing.T) {
	if got, err := ParseRating("explicit"); err != nil || got != RatingExplicit {
		t.Fatalf("ParseRating(explicit) = %q, %v", got, err)
	}

	if got, err := ParseRating("all"); err != nil || got != RatingAll {
		t.Fatalf("ParseRating(all) = %q, %v", got, err)
	}

	if _, err := ParseRating("nsfw"); err == nil {
		t.Error("ParseRating(nsfw) error = nil, want an error")
	}
}

func TestRatingRank(t *testing.T) {
	if RatingGeneral.Rank() >= RatingExplicit.Rank() {
		t.Fatal("general should rank below explicit")
	}

	if RatingAll.Rank() <= RatingExplicit.Rank() {
		t.Fatal("all should rank above explicit so it never narrows a cap")
	}

	if !Rating("").IsUnset() {
		t.Error("an empty rating should be unset")
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		count, max, want int
	}{
		{50, 200, 25},
		{200, 200, 100},
		{0, 200, 0},
		{5, 0, 0},
		{0, 0, 0},
		{-5, 100, 0},
	}

	for _, tt := range tests {
		if got := Percent(tt.count, tt.max); got != tt.want {
			t.Errorf("Percent(%d, %d) = %d, want %d", tt.count, tt.max, got, tt.want)
		}
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register("danbooru", stubProvider{}, ""); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if err := registry.Register("rule34", nil, "RULE34_API_KEY is not set"); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if err := registry.Register("", stubProvider{}, ""); err == nil {
		t.Error("Register(\"\") error = nil, want an error")
	}

	if err := registry.Register("danbooru", stubProvider{}, ""); err == nil {
		t.Error("duplicate Register() error = nil, want an error")
	}

	if err := registry.Register("broken", nil, ""); err == nil {
		t.Error("Register() with no provider and no reason error = nil, want an error")
	}

	if _, err := registry.Get("nope"); err == nil {
		t.Error("Get(nope) error = nil, want an error")
	}

	if entry, err := registry.Get("rule34"); err != nil || entry.Active || entry.Reason == "" {
		t.Errorf("Get(rule34) = %+v, %v, want an inactive entry with a reason", entry, err)
	}

	if active := registry.Active(); len(active) != 1 || active[0].Name != "danbooru" {
		t.Errorf("Active() = %+v, want only danbooru", active)
	}

	if names := registry.Names(); len(names) != 2 {
		t.Errorf("Names() = %v, want 2 entries", names)
	}
}

func TestPostTagsByCategory(t *testing.T) {
	post := Post{Tags: []Tag{
		{Name: "blue_eyes", Category: CategoryGeneral},
		{Name: "somebody", Category: CategoryArtist},
		{Name: "blue_eyes", Category: CategoryGeneral},
	}}

	grouped := post.TagsByCategory()

	if len(grouped[CategoryGeneral]) != 2 {
		t.Errorf("general tags = %v, want 2", grouped[CategoryGeneral])
	}

	if len(grouped[CategoryArtist]) != 1 || grouped[CategoryArtist][0] != "somebody" {
		t.Errorf("artist tags = %v, want [somebody]", grouped[CategoryArtist])
	}

	if grouped[CategoryMeta] != nil {
		t.Errorf("meta tags = %v, want nil", grouped[CategoryMeta])
	}
}
