package booru

import (
	"context"
	"sort"
	"sync"
	"time"
)

// maxImplicationPages bounds a crawl so a misbehaving upstream cannot make Refresh run forever.
const maxImplicationPages = 1000

// ImplicationSource is the one capability the index needs: a page of the active implication graph.
type ImplicationSource interface {
	ImplicationPage(ctx context.Context, page, limit int) ([]Implication, error)
}

// ImplicationIndex is the only ingested dataset in the tool. Danbooru's implication graph is small enough to hold in
// memory and changes slowly, so it is crawled once and refreshed on an interval rather than queried per result. Query
// code only ever reads it, which keeps the crawl off the request path.
type ImplicationIndex struct {
	source   ImplicationSource
	pageSize int
	interval time.Duration

	mu      sync.RWMutex
	byCause map[string][]string
	builtAt time.Time
}

func NewImplicationIndex(source ImplicationSource, interval time.Duration) *ImplicationIndex {
	if interval < 0 {
		interval = 0
	}

	return &ImplicationIndex{
		source:   source,
		pageSize: implicationPageCeiling,
		interval: interval,
		byCause:  map[string][]string{},
	}
}

// Implications returns a copy of the canonical names implied by name, or nil when none are known.
func (i *ImplicationIndex) Implications(name string) []string {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	known := i.byCause[NormalizeTag(name)]
	if len(known) == 0 {
		return nil
	}

	out := make([]string, len(known))
	copy(out, known)

	return out
}

func (i *ImplicationIndex) BuiltAt() time.Time {
	if i == nil {
		return time.Time{}
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.builtAt
}

func (i *ImplicationIndex) Size() int {
	if i == nil {
		return 0
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return len(i.byCause)
}

// Refresh crawls every page of the active implication graph and swaps it in atomically. A crawl that fails part way
// leaves the previous graph untouched.
func (i *ImplicationIndex) Refresh(ctx context.Context) error {
	if i == nil || i.source == nil {
		return nil
	}

	collected := map[string][]string{}

	for page := 1; page <= maxImplicationPages; page++ {
		batch, err := i.source.ImplicationPage(ctx, page, i.pageSize)
		if err != nil {
			return err
		}

		for _, relation := range batch {
			collected[relation.Antecedent] = append(collected[relation.Antecedent], relation.Consequent)
		}

		if len(batch) < i.pageSize {
			break
		}
	}

	for antecedent, consequents := range collected {
		collected[antecedent] = sortedUnique(consequents)
	}

	i.mu.Lock()
	i.byCause = collected
	i.builtAt = time.Now()
	i.mu.Unlock()

	return nil
}

// Start refreshes in the background until ctx is cancelled. An interval of zero disables the index, leaving the graph
// empty and every lookup a miss.
func (i *ImplicationIndex) Start(ctx context.Context, onError func(error)) {
	if i == nil || i.source == nil || i.interval <= 0 {
		return
	}

	go i.run(ctx, onError)
}

func (i *ImplicationIndex) run(ctx context.Context, onError func(error)) {
	refresh := func() {
		if err := i.Refresh(ctx); err != nil && onError != nil {
			onError(err)
		}
	}

	refresh()

	ticker := time.NewTicker(i.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			refresh()
		}
	}
}

func sortedUnique(values []string) []string {
	sort.Strings(values)

	out := values[:0]

	for idx, value := range values {
		if idx > 0 && value == values[idx-1] {
			continue
		}

		out = append(out, value)
	}

	return out
}
