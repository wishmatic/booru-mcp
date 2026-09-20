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

type tagsInput struct {
	Search string `json:"search" jsonschema:"a literal substring to match against Danbooru tag names; spaces and underscores are equivalent"`
	Offset *int   `json:"offset,omitempty" jsonschema:"optional: how many matching tags to skip; defaults to 0"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"optional: how many matching tags to return; defaults to 25"`
}

type tagsOutput struct {
	Search string      `json:"search"`
	Offset int         `json:"offset"`
	Limit  int         `json:"limit"`
	More   bool        `json:"more"`
	Tags   []tagOutput `json:"tags"`
}

type tagOutput struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}

func registerTags(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "tags",
		Description: "Search Danbooru tag names and return the matching tags ordered by work count. `search` is matched " +
			"as a literal substring of the tag name, so `blue hair` and `blue_hair` are the same search. The response " +
			"is one page of the match list: use `offset` and `limit` to walk it and `more` to tell whether another page " +
			"exists. An empty `tags` array with `more` false means no tags match.",
		InputSchema: tagsSchema(),
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

	h.log.Debug("tool called",
		zap.String("tool", "tags"),
		zap.String("search", in.Search),
		zap.Int("offset", offset),
		zap.Int("limit", limit),
	)

	result, err := h.catalog.Tags(ctx, catalog.TagsInput{Search: in.Search, Offset: offset, Limit: limit})
	if err != nil {
		h.log.Error("tags failed", zap.Error(err))

		return nil, tagsOutput{}, fmt.Errorf("tags: %w", err)
	}

	out := tagsOutput{
		Search: result.Search,
		Offset: offset,
		Limit:  limit,
		More:   result.More,
		Tags:   toTagOutputs(result.Tags),
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderTags(out)}}}, out, nil
}

func tagsSchema() *jsonschema.Schema {
	schema := schemaFor[tagsInput]("tags")
	schema.Required = []string{"search"}
	setDefault(schema, "offset", 0)
	setDefault(schema, "limit", catalog.DefaultTagLimit)

	return schema
}

func toTagOutputs(tags []booru.Tag) []tagOutput {
	out := make([]tagOutput, 0, len(tags))

	for _, tag := range tags {
		out = append(out, tagOutput{Name: tag.Name, Category: tag.Category.String(), Count: tag.Count})
	}

	return out
}
