package booru

import (
	"sort"
	"sync"
	"time"
)

// RelationIndex is the only ingested dataset in the tool. Danbooru's implication and alias graphs are small enough to
// hold in memory and change slowly, so they are crawled once and refreshed on an interval rather than queried per
// result. Query code only ever reads the index, which keeps the crawl off the request path.
type RelationIndex struct {
	source   RelationSource
	pageSize int
	interval time.Duration

	mu        sync.RWMutex
	byCause   map[string][]string
	aliasOf   map[string]string
	aliasesOf map[string][]string
	builtAt   time.Time
}

func NewRelationIndex(source RelationSource, interval time.Duration) *RelationIndex {
	if interval < 0 {
		interval = 0
	}

	return &RelationIndex{
		source:    source,
		pageSize:  implicationPageCeiling,
		interval:  interval,
		byCause:   map[string][]string{},
		aliasOf:   map[string]string{},
		aliasesOf: map[string][]string{},
	}
}

// Implications returns a copy of the canonical names implied by name, or nil when none are known.
func (i *RelationIndex) Implications(name string) []string {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return copyNames(i.byCause[NormalizeTag(name)])
}

// Canonical follows the active alias chain from name to the first name that is not itself aliased. It reports false
// when name is not an alias.
func (i *RelationIndex) Canonical(name string) (string, bool) {
	if i == nil {
		return "", false
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.canonicalLocked(NormalizeTag(name))
}

// Synonyms returns the canonical names this concept is also known by: the alias target when name is an alias, and the
// active aliases of name and of its target. It is the recall half of alias resolution; the names are canonical tags,
// not substring matches.
func (i *RelationIndex) Synonyms(name string) []string {
	if i == nil {
		return nil
	}

	key := NormalizeTag(name)
	if key == "" {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	known := map[string]bool{}

	if target, ok := i.canonicalLocked(key); ok {
		known[target] = true

		for _, sibling := range i.aliasesOf[target] {
			known[sibling] = true
		}
	}

	for _, alias := range i.aliasesOf[key] {
		known[alias] = true
	}

	delete(known, key)

	return keys(known)
}

// Ready reports whether the graph has been built at least once, so a caller can tell "no such alias" from "not crawled
// yet".
func (i *RelationIndex) Ready() bool {
	if i == nil {
		return false
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return !i.builtAt.IsZero()
}

func (i *RelationIndex) BuiltAt() time.Time {
	if i == nil {
		return time.Time{}
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return i.builtAt
}

func (i *RelationIndex) Size() int {
	if i == nil {
		return 0
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	return len(i.byCause)
}

func (i *RelationIndex) canonicalLocked(key string) (string, bool) {
	if key == "" {
		return "", false
	}

	seen := map[string]bool{key: true}
	aliased := false

	for {
		target, ok := i.aliasOf[key]
		if !ok || seen[target] {
			break
		}

		aliased = true
		seen[target] = true
		key = target
	}

	if !aliased {
		return "", false
	}

	return key, true
}

func copyNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, len(values))
	copy(out, values)

	return out
}

func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))

	for value := range set {
		out = append(out, value)
	}

	sort.Strings(out)

	return out
}
