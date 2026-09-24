package booru

import (
	"context"
	"errors"
	"testing"
)

type stubImplicationSource struct {
	pages   map[int][]Implication
	err     error
	errPage int
}

func (s *stubImplicationSource) ImplicationPage(_ context.Context, page, _ int) ([]Implication, error) {
	if s.err != nil && page == s.errPage {
		return nil, s.err
	}

	relations := s.pages[page]
	if len(relations) == 0 {
		return []Implication{}, nil
	}

	return relations, nil
}

func TestImplicationIndexRefreshMergesPages(t *testing.T) {
	source := &stubImplicationSource{pages: map[int][]Implication{
		1: {{Antecedent: "futanari_pov", Consequent: "pov"}, {Antecedent: "futanari_pov", Consequent: "futanari"}},
		2: {{Antecedent: "futanari_pov", Consequent: "futanari"}, {Antecedent: "solo_focus", Consequent: "solo"}},
	}}

	index := NewImplicationIndex(source, 0)
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

	if index.Size() != 2 {
		t.Errorf("size = %d, want 2", index.Size())
	}

	if index.BuiltAt().IsZero() {
		t.Error("BuiltAt() is zero after a successful refresh")
	}
}

func TestImplicationIndexKeepsPreviousGraphOnFailure(t *testing.T) {
	source := &stubImplicationSource{pages: map[int][]Implication{1: {{Antecedent: "a", Consequent: "b"}}}}

	index := NewImplicationIndex(source, 0)
	index.pageSize = 1

	if err := index.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}

	if got := index.Implications("a"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("implications = %v, want [b] after a successful crawl", got)
	}

	source.err = errors.New("upstream down")
	source.errPage = 2

	if err := index.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() error = nil, want the failed page reported")
	}

	if got := index.Implications("a"); len(got) != 1 || got[0] != "b" {
		t.Errorf("implications = %v, want the previous graph kept after a failed crawl", got)
	}
}

func TestImplicationIndexDisabledStartDoesNotCrawl(t *testing.T) {
	source := &stubImplicationSource{pages: map[int][]Implication{1: {{Antecedent: "a", Consequent: "b"}}}}

	index := NewImplicationIndex(source, 0)
	index.Start(context.Background(), func(error) { t.Error("the index reported an error while disabled") })

	if index.Size() != 0 {
		t.Errorf("size = %d, want 0 while disabled", index.Size())
	}
}

func TestNilImplicationIndexIsSafe(t *testing.T) {
	var index *ImplicationIndex

	if index.Implications("a") != nil || index.Size() != 0 || !index.BuiltAt().IsZero() {
		t.Error("a nil index should answer safely")
	}

	if err := index.Refresh(context.Background()); err != nil {
		t.Errorf("Refresh() error: %v", err)
	}
}
