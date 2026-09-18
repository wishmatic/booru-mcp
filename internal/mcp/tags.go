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

type tagsInput struct {
	clientsInput
	categoryInput

	Query   string `json:"query" jsonschema:"a substring to match against tag names"`
	Limit   int    `json:"limit,omitempty" jsonschema:"optional: maximum number of tags to return; defaults to 25"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"optional: re-fetch from the clients even if the cache is fresh"`
}

type tagsOutput struct {
	Tags     []tagOutput     `json:"tags"`
	Skipped  []skippedOutput `json:"skipped,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

func registerTags(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "tags",
		Description: "Search tags across the requested clients and return them sorted by popularity. Popularity is a " +
			"percentage within this query: each client's strongest match is 100%, so a result can be compared across " +
			"clients even though their raw work counts are not comparable. Every result shows the client it came from " +
			"and that client's raw work count. Results are cached with a long TTL.",
		InputSchema: tagsSchema(),
	}, h.tags)
}

func (h *handlers) tags(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in tagsInput,
) (*mcp.CallToolResult, tagsOutput, error) {
	category, err := h.parseCategory(in.Category)
	if err != nil {
		return nil, tagsOutput{}, fmt.Errorf("tags: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "tags"),
		zap.String("query", in.Query),
		zap.Strings("clients", in.Clients),
		zap.String("category", in.Category),
		zap.Int("limit", in.Limit),
		zap.Bool("refresh", in.Refresh),
	)

	result, err := h.catalog.Tags(ctx, catalog.TagsInput{
		Query:    in.Query,
		Clients:  in.Clients,
		Category: category,
		Limit:    in.Limit,
		Refresh:  in.Refresh,
	})
	if err != nil {
		h.log.Error("tags failed", zap.Error(err))

		return nil, tagsOutput{}, fmt.Errorf("tags: %w", err)
	}

	out := tagsOutput{
		Tags:     toTagOutputs(result.Tags),
		Skipped:  toSkippedOutputs(result.Skipped),
		Warnings: result.Warnings,
	}

	text := present.Tags(result.Tags)

	if note := present.Skipped(result.Skipped); note != "" {
		text += "\n" + note
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func tagsSchema() *jsonschema.Schema {
	schema := schemaFor[tagsInput]("tags")
	setCategoryEnum(schema)
	setDefault(schema, "limit", 25)
	setDefault(schema, "refresh", false)

	return schema
}
