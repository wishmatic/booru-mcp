package catalog

import (
	"sort"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

const rrfK = 60

type tagSet struct {
	Client string
	Tags   []booru.Tag
}

type rankedTag struct {
	tag    booru.Tag
	client string
	rank   int
	pct    int
}

func rankSet(set tagSet) []rankedTag {
	sorted := make([]booru.Tag, len(set.Tags))
	copy(sorted, set.Tags)

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}

		return sorted[i].Name < sorted[j].Name
	})

	highest := 0
	for _, tag := range sorted {
		if tag.Count > highest {
			highest = tag.Count
		}
	}

	ranked := make([]rankedTag, 0, len(sorted))

	for i, tag := range sorted {
		ranked = append(ranked, rankedTag{
			tag:    tag,
			client: set.Client,
			rank:   i + 1,
			pct:    booru.Percent(tag.Count, highest),
		})
	}

	return ranked
}

// mergeTagSets deduplicates by name across clients, keeping the instance with the highest within-query percentage, and
// sorts the result by that percentage. It backs `tags`.
func mergeTagSets(sets []tagSet) []booru.FusedTag {
	type accumulator struct {
		category   booru.TagCategory
		bestClient string
		bestPct    int
		bestCount  int
		total      int
		clients    []booru.TagCount
	}

	byName := make(map[string]*accumulator)

	for _, set := range sets {
		for _, ranked := range rankSet(set) {
			entry, ok := byName[ranked.tag.Name]
			if !ok {
				entry = &accumulator{}
				byName[ranked.tag.Name] = entry
			}

			entry.total += ranked.tag.Count
			entry.clients = append(entry.clients, booru.TagCount{
				Client: ranked.client,
				Count:  ranked.tag.Count,
				Rank:   ranked.rank,
				Pct:    ranked.pct,
			})

			if entry.bestClient == "" || ranked.pct > entry.bestPct {
				entry.category = ranked.tag.Category
				entry.bestClient = ranked.client
				entry.bestPct = ranked.pct
				entry.bestCount = ranked.tag.Count
			}
		}
	}

	fused := make([]booru.FusedTag, 0, len(byName))

	for name, entry := range byName {
		fused = append(fused, booru.FusedTag{
			Name:     name,
			Category: entry.category,
			Score:    float64(entry.bestPct),
			Total:    entry.total,
			Client:   entry.bestClient,
			Count:    entry.bestCount,
			Clients:  entry.clients,
		})
	}

	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].Score != fused[j].Score {
			return fused[i].Score > fused[j].Score
		}

		if fused[i].Count != fused[j].Count {
			return fused[i].Count > fused[j].Count
		}

		return fused[i].Name < fused[j].Name
	})

	return fused
}

// Fuse combines per-client tag lists with reciprocal rank fusion. Counts from different corpora are not comparable, so
// only the ordering within each client is used, and each client's raw count and percentage travel alongside.
func Fuse(perClient map[string][]booru.Tag) []booru.FusedTag {
	clients := make([]string, 0, len(perClient))
	for client := range perClient {
		clients = append(clients, client)
	}

	sort.Strings(clients)

	type accumulator struct {
		category booru.TagCategory
		score    float64
		total    int
		maxCount int
		clients  []booru.TagCount
	}

	byName := make(map[string]*accumulator)

	for _, client := range clients {
		for _, ranked := range rankSet(tagSet{Client: client, Tags: perClient[client]}) {
			entry, ok := byName[ranked.tag.Name]
			if !ok {
				entry = &accumulator{category: ranked.tag.Category}
				byName[ranked.tag.Name] = entry
			}

			entry.score += 1 / float64(rrfK+ranked.rank)
			entry.total += ranked.tag.Count

			if ranked.tag.Count > entry.maxCount {
				entry.maxCount = ranked.tag.Count
			}

			entry.clients = append(entry.clients, booru.TagCount{
				Client: ranked.client,
				Count:  ranked.tag.Count,
				Rank:   ranked.rank,
				Pct:    ranked.pct,
			})
		}
	}

	fused := make([]booru.FusedTag, 0, len(byName))

	for name, entry := range byName {
		fused = append(fused, booru.FusedTag{
			Name:     name,
			Category: entry.category,
			Score:    entry.score,
			Total:    entry.total,
			Count:    entry.maxCount,
			Clients:  entry.clients,
		})
	}

	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].Score != fused[j].Score {
			return fused[i].Score > fused[j].Score
		}

		if fused[i].Total != fused[j].Total {
			return fused[i].Total > fused[j].Total
		}

		return fused[i].Name < fused[j].Name
	})

	return fused
}
