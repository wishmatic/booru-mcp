package danbooru

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func newTierClient(t *testing.T, tagLimit int, tier string) (*Client, *string) {
	t.Helper()

	var got string

	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("tags")
		_, _ = w.Write([]byte(`[]`))
	})
	client.tagLimit = tagLimit
	client.tier = tier

	return client, &got
}

func TestSearchAtTagLimitSucceeds(t *testing.T) {
	client, got := newTierClient(t, 2, "anonymous")

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "cat_ears 1girl"}); err != nil {
		t.Fatalf("Search() error: %v, want a two-tag query at the cap to succeed", err)
	}

	if *got != "cat_ears 1girl" {
		t.Errorf("tags = %q, want exactly the two user tags", *got)
	}
}

func TestSearchAboveTagLimitNamesTheCap(t *testing.T) {
	client, _ := newTierClient(t, 2, "anonymous")

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "cat_ears 1girl solo"})
	if err == nil {
		t.Fatal("Search() error = nil, want an over-cap query to fail")
	}

	for _, want := range []string{"3 search terms", "anonymous", "limit of 2", "DANBOORU_LOGIN", `terms sent: "cat_ears 1girl solo"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestSearchTagLimitCountsRandomButNotRating(t *testing.T) {
	client, _ := newTierClient(t, 2, "anonymous")

	if _, err := client.Search(context.Background(), booru.SearchParams{
		Tags:   "cat_ears",
		Rating: booru.RatingExplicit,
		Random: true,
	}); err != nil {
		t.Fatalf("Search() error: %v, want rating excluded and one tag plus order:random to fit", err)
	}

	_, err := client.Search(context.Background(), booru.SearchParams{
		Tags:   "cat_ears 1girl",
		Random: true,
	})
	if err == nil {
		t.Fatal("Search() error = nil, want order:random to count toward the cap")
	}
}

func TestSearchAppendsNoAutomaticNegatives(t *testing.T) {
	client, got := newTierClient(t, 0, "gold")

	if _, err := client.Search(context.Background(), booru.SearchParams{Tags: "cat_ears"}); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if *got != "cat_ears" {
		t.Fatalf("tags = %q, want no automatic negative terms", *got)
	}

	for _, term := range strings.Fields(*got) {
		if strings.HasPrefix(term, "-") {
			t.Errorf("tags = %q contains an automatic negative %q", *got, term)
		}
	}
}
