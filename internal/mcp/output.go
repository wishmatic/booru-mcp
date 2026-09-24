package mcp

import "github.com/wishmatic/booru-mcp/internal/booru"

type tagsOutput struct {
	Search       string      `json:"search"`
	Exact        bool        `json:"exact"`
	Status       string      `json:"status"`
	SnapshotDate string      `json:"snapshot_date"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
	MinCount     int         `json:"min_count"`
	More         bool        `json:"more"`
	AliasOf      string      `json:"alias_of"`
	Withheld     int         `json:"withheld"`
	WithheldBest int         `json:"withheld_best_count"`
	Synonyms     []string    `json:"synonyms"`
	Tags         []tagOutput `json:"tags"`
}

type tagOutput struct {
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Count         int      `json:"count"`
	CountIsTarget bool     `json:"count_is_target"`
	AliasOf       string   `json:"alias_of"`
	Implications  []string `json:"implications"`
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}

	return values
}

func toTagOutputs(tags []booru.Tag) []tagOutput {
	out := make([]tagOutput, 0, len(tags))

	for _, tag := range tags {
		out = append(out, tagOutput{
			Name:          tag.Name,
			Category:      tag.Category.String(),
			Count:         tag.Count,
			CountIsTarget: tag.CountIsTarget,
			AliasOf:       tag.AliasOf,
			Implications:  nonNilStrings(tag.Implications),
		})
	}

	return out
}
