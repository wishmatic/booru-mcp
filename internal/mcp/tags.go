package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"go.uber.org/zap"
)

type handlers struct {
	log     *zap.Logger
	catalog *catalog.Service
}

// DefaultMinCount keeps zero-work canonical rows out of substring results unless a caller asks for them.
const DefaultMinCount = 1

type tagsInput struct {
	Search     string   `json:"search" jsonschema:"tag name text to match; spaces and underscores are equivalent. A literal substring by default; the whole tag name when exact is true"`
	Offset     *int     `json:"offset,omitempty" jsonschema:"optional: how many matching tags to skip; defaults to 0"`
	Limit      *int     `json:"limit,omitempty" jsonschema:"optional: how many matching tags to return; defaults to 25"`
	Exact      *bool    `json:"exact,omitempty" jsonschema:"optional: when true, search is the whole tag name and the result is that one tag or nothing; aliases resolve to their target. Defaults to false"`
	MinCount   *int     `json:"min_count,omitempty" jsonschema:"optional: lowest work count a substring match may have; defaults to 1, and 0 includes the zero-work canonical tail"`
	Categories []string `json:"category,omitempty" jsonschema:"optional: keep only these categories; any of general, artist, copyright, character, meta"`
}

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

func registerTags(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "tags",
		Description: "Search Danbooru tag names and return the matching tags ordered by work count.\n" +
			"- By default `search` is a literal substring of the tag name, so `blue hair` and `blue_hair` are the same " +
			"search. Substring results exclude tags below `min_count` works (default 1); Danbooru's user-editable tag " +
			"table also holds concatenated and punctuation-mangled names, and those are usually the zero-work tail. A " +
			"withheld tag still exists: `withheld` and `withheld_best_count` report what the floor removed, and " +
			"`min_count: 0` returns it.\n" +
			"- Set `exact` to true to ask whether one exact tag exists. This is the only reliable existence check: a " +
			"substring miss proves nothing about whether a real tag exists. Exact mode resolves aliases and reports the " +
			"target's count.\n" +
			"- Each result carries `implications` (canonical tags it implies, empty when none are known), `alias_of` " +
			"(set when the name is an alias), and `count_is_target` (set when `count` is the alias target's, not the " +
			"named tag's own).\n" +
			"- Counts are read live; `snapshot_date` is the date they were read. Comparisons of counts across calls can " +
			"differ because the underlying data changes.\n" +
			"- The response is one page: use `offset` and `limit` to walk it and `more` to tell whether another page " +
			"exists.\n" +
			"- `status` distinguishes a real empty page (`no_substring_match`, `exact_not_found`) from an unconfirmed " +
			"one (`unknown`).",
		InputSchema: tagsSchema(),
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: new(false),
			IdempotentHint:  true,
			OpenWorldHint:   new(true),
		},
	}, h.tags)
}

func (h *handlers) tags(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in tagsInput,
) (*mcp.CallToolResult, tagsOutput, error) {
	offset := 0
	if in.Offset != nil {
		offset = *in.Offset
	}

	limit := catalog.DefaultTagLimit
	if in.Limit != nil {
		limit = *in.Limit
	}

	minCount := DefaultMinCount
	if in.MinCount != nil {
		minCount = *in.MinCount
	}

	exact := in.Exact != nil && *in.Exact

	categories, err := parseCategories(in.Categories)
	if err != nil {
		return nil, tagsOutput{}, fmt.Errorf("tags: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "tags"),
		zap.String("search", in.Search),
		zap.Int("offset", offset),
		zap.Int("limit", limit),
		zap.Int("min_count", minCount),
		zap.Bool("exact", exact),
	)

	result, err := h.catalog.Tags(ctx, catalog.TagsInput{
		Search:     in.Search,
		Offset:     offset,
		Limit:      limit,
		MinCount:   minCount,
		Exact:      exact,
		Categories: categories,
	})
	if err != nil {
		h.log.Error("tags failed", zap.Error(err))

		return nil, tagsOutput{}, fmt.Errorf("tags: %w", err)
	}

	out := tagsOutput{
		Search:       result.Search,
		Exact:        exact,
		Status:       string(result.Status),
		SnapshotDate: result.SnapshotDate,
		Offset:       offset,
		Limit:        limit,
		MinCount:     minCount,
		More:         result.More,
		AliasOf:      result.AliasOf,
		Withheld:     result.Withheld,
		WithheldBest: result.WithheldBest,
		Tags:         toTagOutputs(result.Tags),
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderTags(out)}}}, out, nil
}

func parseCategories(names []string) ([]booru.TagCategory, error) {
	categories := make([]booru.TagCategory, 0, len(names))

	for _, name := range names {
		category, ok := booru.ParseTagCategory(name)
		if !ok {
			return nil, fmt.Errorf("category %q is not one of %s", name, booru.CategoryList())
		}

		categories = append(categories, category)
	}

	return categories, nil
}

func tagsSchema() *jsonschema.Schema {
	schema := schemaFor[tagsInput]("tags")
	schema.Required = []string{"search"}
	setDefault(schema, "offset", 0)
	setDefault(schema, "limit", catalog.DefaultTagLimit)
	setDefault(schema, "min_count", DefaultMinCount)
	setDefault(schema, "exact", false)
	setCategoryEnum(schema)

	return schema
}

func setCategoryEnum(schema *jsonschema.Schema) {
	property, ok := schema.Properties["category"]
	if !ok || property.Items == nil {
		return
	}

	allowed := make([]any, 0, len(booru.CategoryNames()))

	for _, name := range booru.CategoryNames() {
		allowed = append(allowed, name.String())
	}

	property.Items.Enum = allowed
}

func toTagOutputs(tags []booru.Tag) []tagOutput {
	out := make([]tagOutput, 0, len(tags))

	for _, tag := range tags {
		implications := tag.Implications
		if implications == nil {
			implications = []string{}
		}

		out = append(out, tagOutput{
			Name:          tag.Name,
			Category:      tag.Category.String(),
			Count:         tag.Count,
			CountIsTarget: tag.CountIsTarget,
			AliasOf:       tag.AliasOf,
			Implications:  implications,
		})
	}

	return out
}
