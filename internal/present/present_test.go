package present

import (
	"strings"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func TestTags(t *testing.T) {
	out := Tags([]booru.FusedTag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Score: 100, Count: 900, Client: "danbooru"},
		{Name: "smile", Category: booru.CategoryGeneral, Score: 50, Count: 450, Client: "danbooru"},
	})

	for _, want := range []string{"blue_eyes", "100%", "900", "danbooru", "smile", "50%"} {
		if !strings.Contains(out, want) {
			t.Errorf("Tags() = %q, missing %q", out, want)
		}
	}

	if got := Tags(nil); got != "No tags matched." {
		t.Errorf("Tags(nil) = %q", got)
	}
}

func TestRelatedStatesPartialCoverage(t *testing.T) {
	out := Related(
		[]booru.RelatedTag{{Tag: "blue_eyes", Client: "danbooru", Score: 900}},
		[]booru.Skipped{{Client: "yandere", Reason: "the client has no related-tag API"}},
	)

	for _, want := range []string{"blue_eyes", "yandere", "no related-tag API", "shorter than requested"} {
		if !strings.Contains(out, want) {
			t.Errorf("Related() = %q, missing %q", out, want)
		}
	}

	empty := Related(nil, nil)
	if !strings.Contains(empty, "expected rather than a failure") {
		t.Errorf("Related(nil, nil) = %q, want it to say an empty result is expected", empty)
	}
}

func TestPostsAndPost(t *testing.T) {
	post := booru.Post{
		Client:    "danbooru",
		ID:        "5",
		URL:       "https://danbooru.donmai.us/posts/5",
		FileURL:   "https://cdn.example.com/5.png",
		Width:     800,
		Height:    600,
		Rating:    booru.RatingGeneral,
		Score:     10,
		FavCount:  3,
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		Tags:      []booru.Tag{{Name: "somebody", Category: booru.CategoryArtist}, {Name: "smile", Category: booru.CategoryGeneral}},
	}

	list := Posts([]booru.Post{post})

	for _, want := range []string{"danbooru:5", "https://danbooru.donmai.us/posts/5", "800x600", "smile"} {
		if !strings.Contains(list, want) {
			t.Errorf("Posts() = %q, missing %q", list, want)
		}
	}

	detail := Post(post)

	for _, want := range []string{"danbooru:5", "## artist", "## general", "favourites: 3"} {
		if !strings.Contains(detail, want) {
			t.Errorf("Post() = %q, missing %q", detail, want)
		}
	}

	if got := Posts(nil); got != "No posts matched." {
		t.Errorf("Posts(nil) = %q", got)
	}
}
