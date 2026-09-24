package booru

import (
	"context"
	"errors"
	"testing"
)

type stubRelationSource struct {
	implications map[int][]Implication
	aliases      map[int][]Alias
	err          error
	errPage      int
}

func (s *stubRelationSource) ImplicationPage(_ context.Context, page, _ int) ([]Implication, error) {
	if s.err != nil && page == s.errPage {
		return nil, s.err
	}

	return orEmptyImplications(s.implications[page]), nil
}

func (s *stubRelationSource) AliasPage(_ context.Context, page, _ int) ([]Alias, error) {
	if s.err != nil && page == s.errPage {
		return nil, s.err
	}

	if aliases := s.aliases[page]; len(aliases) > 0 {
		return aliases, nil
	}

	return []Alias{}, nil
}

func orEmptyImplications(relations []Implication) []Implication {
	if relations == nil {
		return []Implication{}
	}

	return relations
}

func TestRelationIndexRefreshMergesPages(t *testing.T) {
	source := &stubRelationSource{
		implications: map[int][]Implication{
			1: {{Antecedent: "futanari_pov", Consequent: "pov"}, {Antecedent: "futanari_pov", Consequent: "futanari"}},
			2: {{Antecedent: "futanari_pov", Consequent: "futanari"}, {Antecedent: "solo_focus", Consequent: "solo"}},
		},
		aliases: map[int][]Alias{
			1: {{Antecedent: "piss", Consequent: "pee"}, {Antecedent: "urine", Consequent: "pee"}},
			2: {{Antecedent: "urination", Consequent: "peeing"}},
		},
	}

	index := NewRelationIndex(source, 0)
	index.pageSize = 2

	if err := index.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if got := index.Implications("futanari_pov"); len(got) != 2 || got[0] != "futanari" || got[1] != "pov" {
		t.Errorf("implications = %v, want deduplicated and sorted", got)
	}

	if got := index.Implications("solo_focus"); len(got) != 1 || got[0] != "solo" {
		t.Errorf("implications = %v, want [solo]", got)
	}

	if !index.Ready() {
		t.Error("Ready() = false after a successful refresh")
	}

	if index.BuiltAt().IsZero() {
		t.Error("BuiltAt() is zero after a successful refresh")
	}
}

func TestRelationIndexCanonicalAndSynonyms(t *testing.T) {
	source := &stubRelationSource{aliases: map[int][]Alias{
		1: {
			{Antecedent: "piss", Consequent: "pee"},
			{Antecedent: "urine", Consequent: "pee"},
			{Antecedent: "urination", Consequent: "peeing"},
			{Antecedent: "peeing", Consequent: "peeing_canonical"},
		},
	}}

	index := NewRelationIndex(source, 0)
	index.pageSize = 1000

	if err := index.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	target, aliased := index.Canonical("piss")
	if !aliased || target != "pee" {
		t.Errorf("Canonical(piss) = %q aliased %v, want pee", target, aliased)
	}

	chained, aliased := index.Canonical("urination")
	if !aliased || chained != "peeing_canonical" {
		t.Errorf("Canonical(urination) = %q aliased %v, want the chain terminal", chained, aliased)
	}

	if _, aliased := index.Canonical("pee"); aliased {
		t.Error("Canonical(pee) aliased = true, want pee to be canonical")
	}

	got := index.Synonyms("piss")
	want := []string{"pee", "urine"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Synonyms(piss) = %v, want %v", got, want)
	}

	siblings := index.Synonyms("pee")
	if len(siblings) != 2 || siblings[0] != "piss" || siblings[1] != "urine" {
		t.Errorf("Synonyms(pee) = %v, want its aliases", siblings)
	}

	if got := index.Synonyms("no_such_tag"); len(got) != 0 {
		t.Errorf("Synonyms(no_such_tag) = %v, want none", got)
	}
}

func TestRelationIndexKeepsPreviousGraphsOnFailure(t *testing.T) {
	source := &stubRelationSource{
		implications: map[int][]Implication{1: {{Antecedent: "a", Consequent: "b"}}},
		aliases:      map[int][]Alias{1: {{Antecedent: "piss", Consequent: "pee"}}},
	}

	index := NewRelationIndex(source, 0)
	index.pageSize = 1

	if err := index.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if target, aliased := index.Canonical("piss"); !aliased || target != "pee" {
		t.Fatalf("Canonical(piss) = %q aliased %v, want pee after a successful crawl", target, aliased)
	}

	source.err = errors.New("upstream down")
	source.errPage = 2

	if err := index.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() error = nil, want the failed page reported")
	}

	if got := index.Implications("a"); len(got) != 1 || got[0] != "b" {
		t.Errorf("implications = %v, want the previous graph kept after a failed crawl", got)
	}

	if target, aliased := index.Canonical("piss"); !aliased || target != "pee" {
		t.Errorf("Canonical(piss) = %q aliased %v, want the previous alias graph kept", target, aliased)
	}
}

func TestRelationIndexDisabledStartDoesNotCrawl(t *testing.T) {
	source := &stubRelationSource{implications: map[int][]Implication{1: {{Antecedent: "a", Consequent: "b"}}}}

	index := NewRelationIndex(source, 0)
	index.Start(context.Background(), func(error) { t.Error("the index reported an error while disabled") })

	if index.Size() != 0 || index.Ready() {
		t.Errorf("size = %d ready = %v, want an empty index while disabled", index.Size(), index.Ready())
	}
}

func TestNilRelationIndexIsSafe(t *testing.T) {
	var index *RelationIndex

	if index.Implications("a") != nil || index.Synonyms("a") != nil || index.Size() != 0 || index.Ready() {
		t.Error("a nil index should answer safely")
	}

	if _, aliased := index.Canonical("a"); aliased {
		t.Error("a nil index should report no alias")
	}

	if !index.BuiltAt().IsZero() {
		t.Error("a nil index should have no build time")
	}

	if err := index.Refresh(context.Background()); err != nil {
		t.Errorf("Refresh() error: %v", err)
	}
}
