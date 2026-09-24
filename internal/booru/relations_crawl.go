package booru

import (
	"context"
	"sort"
	"time"
)

// maxRelationPages bounds a crawl so a misbehaving upstream cannot make Refresh run forever.
const maxRelationPages = 1000

// RelationSource is the two canonical tables the index ingests: the active implication graph and the active alias
// graph.
type RelationSource interface {
	ImplicationPage(ctx context.Context, page, limit int) ([]Implication, error)
	AliasPage(ctx context.Context, page, limit int) ([]Alias, error)
}

// Refresh crawls every page of both graphs and swaps them in atomically. A crawl that fails part way leaves the
// previous graphs untouched.
func (i *RelationIndex) Refresh(ctx context.Context) error {
	if i == nil || i.source == nil {
		return nil
	}

	byCause, err := i.crawlImplications(ctx)
	if err != nil {
		return err
	}

	aliasOf, aliasesOf, err := i.crawlAliases(ctx)
	if err != nil {
		return err
	}

	i.mu.Lock()
	i.byCause = byCause
	i.aliasOf = aliasOf
	i.aliasesOf = aliasesOf
	i.builtAt = time.Now()
	i.mu.Unlock()

	return nil
}

// Start refreshes in the background until ctx is cancelled. An interval of zero disables the index, leaving the graphs
// empty and every lookup a miss.
func (i *RelationIndex) Start(ctx context.Context, onError func(error)) {
	if i == nil || i.source == nil || i.interval <= 0 {
		return
	}

	go i.run(ctx, onError)
}

func (i *RelationIndex) run(ctx context.Context, onError func(error)) {
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

func (i *RelationIndex) crawlImplications(ctx context.Context) (map[string][]string, error) {
	collected := map[string][]string{}

	for page := 1; page <= maxRelationPages; page++ {
		batch, err := i.source.ImplicationPage(ctx, page, i.pageSize)
		if err != nil {
			return nil, err
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

	return collected, nil
}

func (i *RelationIndex) crawlAliases(ctx context.Context) (map[string]string, map[string][]string, error) {
	aliasOf := map[string]string{}
	aliasesOf := map[string][]string{}

	for page := 1; page <= maxRelationPages; page++ {
		batch, err := i.source.AliasPage(ctx, page, i.pageSize)
		if err != nil {
			return nil, nil, err
		}

		for _, alias := range batch {
			if _, seen := aliasOf[alias.Antecedent]; seen {
				continue
			}

			aliasOf[alias.Antecedent] = alias.Consequent
			aliasesOf[alias.Consequent] = append(aliasesOf[alias.Consequent], alias.Antecedent)
		}

		if len(batch) < i.pageSize {
			break
		}
	}

	for consequent := range aliasesOf {
		aliasesOf[consequent] = sortedUnique(aliasesOf[consequent])
	}

	return aliasOf, aliasesOf, nil
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
