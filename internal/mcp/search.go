package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"github.com/wishmatic/booru-mcp/internal/present"
	"go.uber.org/zap"
)

type searchInput struct {
	clientsInput
	ratingInput

	Tags   string `json:"tags" jsonschema:"a booru tag string, for example blue_eyes smile"`
	Limit  int    `json:"limit,omitempty" jsonschema:"optional: maximum posts per client; defaults to 20"`
	Page   int    `json:"page,omitempty" jsonschema:"optional: 1-based page number; defaults to 1"`
	Random bool   `json:"random,omitempty" jsonschema:"optional: ask clients that support it for a random ordering"`
}

type searchOutput struct {
	Posts    []postOutput    `json:"posts"`
	Skipped  []skippedOutput `json:"skipped,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

func registerSearch(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "search",
		Description: "Search posts by booru tag string across the requested clients. Results are grouped in the " +
			"requested client order, each group keeping that client's native ordering, because post ordering is not " +
			"comparable across sites. Every post is returned as a permalink plus file, sample, and preview URLs; no " +
			"image bytes are returned and nothing here is cached.",
		InputSchema: searchSchema(),
	}, h.search)
}

func (h *handlers) search(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in searchInput,
) (*mcp.CallToolResult, searchOutput, error) {
	rating, err := h.parseRating(in.Rating)
	if err != nil {
		return nil, searchOutput{}, fmt.Errorf("search: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "search"),
		zap.String("tags", in.Tags),
		zap.Strings("clients", in.Clients),
		zap.String("rating", in.Rating),
		zap.Int("limit", in.Limit),
		zap.Int("page", in.Page),
		zap.Bool("random", in.Random),
	)

	result, err := h.catalog.Search(ctx, catalog.SearchInput{
		Tags:    in.Tags,
		Clients: in.Clients,
		Limit:   in.Limit,
		Page:    in.Page,
		Rating:  rating,
		Random:  in.Random,
	})
	if err != nil {
		h.log.Error("search failed", zap.Error(err))

		return nil, searchOutput{}, fmt.Errorf("search: %w", err)
	}

	out := searchOutput{
		Posts:    make([]postOutput, 0, len(result.Posts)),
		Skipped:  toSkippedOutputs(result.Skipped),
		Warnings: result.Warnings,
	}

	for _, post := range result.Posts {
		out.Posts = append(out.Posts, toPostOutput(post, false))
	}

	text := present.Posts(result.Posts)

	if note := present.Skipped(result.Skipped); note != "" {
		text += "\n" + note
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func searchSchema() *jsonschema.Schema {
	schema := schemaFor[searchInput]("search")
	setRatingEnum(schema)
	setDefault(schema, "limit", 20)
	setDefault(schema, "page", 1)
	setDefault(schema, "random", false)

	return schema
}
