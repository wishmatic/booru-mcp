package booru

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestSearchTagsSendsCategories(t *testing.T) {
	log := &requestLog{}

	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Query())
		_, _ = w.Write([]byte(`[]`))
	})

	query := TagQuery{Search: "piss", Limit: 5, Categories: []TagCategory{CategoryGeneral, CategoryArtist}}

	if _, err := client.SearchTags(context.Background(), query); err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if got := log.at(0).Get("search[category]"); got != "0,1" {
		t.Errorf("search[category] = %q, want 0,1", got)
	}
}

func TestSearchTagsOmitsCategoryWhenUnset(t *testing.T) {
	log := &requestLog{}

	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Query())
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.SearchTags(context.Background(), TagQuery{Search: "piss", Limit: 5}); err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if _, ok := log.at(0)["search[category]"]; ok {
		t.Errorf("search[category] was sent for an unfiltered search")
	}
}

func TestSearchTagsWithholdsTheZeroCountTail(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"real_a","category":0,"post_count":10},
			{"name":"real_b","category":0,"post_count":5},
			{"name":"real_c","category":0,"post_count":3},
			{"name":"junk_compound","category":0,"post_count":0},
			{"name":"junk_more","category":0,"post_count":0}
		]`))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "real", Limit: 5, MinCount: 1})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(page.Tags) != 3 || page.Tags[0].Name != "real_a" || page.Tags[2].Name != "real_c" {
		t.Fatalf("tags = %+v, want the three rows at or above the floor", page.Tags)
	}

	if page.Withheld != 2 || page.WithheldBest != 0 {
		t.Errorf("withheld = %d best = %d, want 2 and 0", page.Withheld, page.WithheldBest)
	}

	if page.More {
		t.Error("More = true, want false once the floor drops the listing")
	}
}

func TestSearchTagsReturnsTheZeroCountTailWhenTheFloorIsZero(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"real_a","category":0,"post_count":10},
			{"name":"junk_compound","category":0,"post_count":0}
		]`))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "real", Limit: 5})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(page.Tags) != 2 || page.Tags[1].Name != "junk_compound" || page.Tags[1].Count != 0 {
		t.Fatalf("tags = %+v, want the zero-count row included without a floor", page.Tags)
	}

	if page.Withheld != 0 {
		t.Errorf("withheld = %d, want none at a zero floor", page.Withheld)
	}
}

func TestSearchTagsOrdersEqualCountsByName(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"charlie","category":0,"post_count":5},
			{"name":"alpha","category":0,"post_count":5},
			{"name":"bravo","category":0,"post_count":5}
		]`))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Limit: 3})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := []string{"alpha", "bravo", "charlie"}

	if len(page.Tags) != len(want) {
		t.Fatalf("tags = %+v, want %d", page.Tags, len(want))
	}

	for i, name := range want {
		if page.Tags[i].Name != name {
			t.Errorf("tags[%d] = %q, want %q", i, page.Tags[i].Name, name)
		}
	}
}

func TestFindTag(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"futanari","category":0,"post_count":52264}]`))
	})

	tag, found, err := client.FindTag(context.Background(), "Futanari")
	if err != nil {
		t.Fatalf("FindTag() error: %v", err)
	}

	if !found || tag.Name != "futanari" || tag.Count != 52264 || tag.Category != CategoryGeneral {
		t.Fatalf("tag = %+v found = %v, want the canonical row", tag, found)
	}
}

func TestFindTagMissing(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})

	if _, found, err := client.FindTag(context.Background(), "no_such_tag"); err != nil || found {
		t.Fatalf("FindTag() = found %v error %v, want not found and no error", found, err)
	}
}

func TestFindTagSendsExactName(t *testing.T) {
	log := &requestLog{}

	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Query())
		_, _ = w.Write([]byte(`[]`))
	})

	if _, _, err := client.FindTag(context.Background(), "Blue Hair"); err != nil {
		t.Fatalf("FindTag() error: %v", err)
	}

	if got := log.at(0).Get("search[name]"); got != "blue_hair" {
		t.Errorf("search[name] = %q, want blue_hair", got)
	}
}

func TestAliasTargetFollowsAndTerminates(t *testing.T) {
	aliases := map[string]string{
		"dickgirl": "futanari",
		"loop_a":   "loop_b",
		"loop_b":   "loop_a",
	}

	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tag_aliases.json") {
			t.Errorf("unexpected path %q", r.URL.Path)

			return
		}

		antecedent := r.URL.Query().Get("search[antecedent_name]")
		consequent, ok := aliases[antecedent]

		if !ok || antecedent == "futanari" {
			_, _ = w.Write([]byte(`[]`))

			return
		}

		_, _ = w.Write([]byte(`[{"antecedent_name":"` + antecedent + `","consequent_name":"` + consequent + `"}]`))
	})

	target, aliased, err := client.AliasTarget(context.Background(), "dickgirl")
	if err != nil {
		t.Fatalf("AliasTarget() error: %v", err)
	}

	if !aliased || target != "futanari" {
		t.Errorf("AliasTarget(dickgirl) = %q aliased %v, want futanari", target, aliased)
	}

	looped, aliased, err := client.AliasTarget(context.Background(), "loop_a")
	if err != nil {
		t.Fatalf("AliasTarget() error: %v", err)
	}

	if !aliased || looped != "loop_b" {
		t.Errorf("AliasTarget(loop_a) = %q aliased %v, want a terminating answer", looped, aliased)
	}

	if _, aliased, err := client.AliasTarget(context.Background(), "futanari"); err != nil || aliased {
		t.Errorf("AliasTarget(futanari) aliased %v error %v, want not aliased", aliased, err)
	}
}

func TestImplicationsAreSorted(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tag_implications.json") {
			t.Errorf("unexpected path %q", r.URL.Path)

			return
		}

		_, _ = w.Write([]byte(`[
			{"antecedent_name":"futanari_pov","consequent_name":"pov"},
			{"antecedent_name":"futanari_pov","consequent_name":"futanari"}
		]`))
	})

	got, err := client.Implications(context.Background(), "futanari_pov")
	if err != nil {
		t.Fatalf("Implications() error: %v", err)
	}

	if len(got) != 2 || got[0] != "futanari" || got[1] != "pov" {
		t.Errorf("implications = %v, want [futanari pov]", got)
	}
}

func TestImplicationPageMapsRows(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tag_implications.json") {
			t.Errorf("unexpected path %q", r.URL.Path)

			return
		}

		_, _ = w.Write([]byte(`[{"antecedent_name":"a","consequent_name":"b"},{"antecedent_name":"","consequent_name":"c"}]`))
	})

	relations, err := client.ImplicationPage(context.Background(), 1, 1000)
	if err != nil {
		t.Fatalf("ImplicationPage() error: %v", err)
	}

	if len(relations) != 1 || relations[0] != (Implication{Antecedent: "a", Consequent: "b"}) {
		t.Errorf("relations = %+v, want only the complete row", relations)
	}
}
