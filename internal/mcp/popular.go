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

type popularInput struct {
	clientsInput
	categoryInput

	Limit   int  `json:"limit,omitempty" jsonschema:"optional: maximum number of tags to return; defaults to 25"`
	Refresh bool `json:"refresh,omitempty" jsonschema:"optional: re-fetch the popular snapshot even if it is fresh"`
}

type popularOutput struct {
	Tags     []tagOutput          `json:"tags"`
	Skipped  []skippedOutput      `json:"skipped,omitempty"`
	Warnings []string             `json:"warnings,omitempty"`
	Clients  []clientStatusOutput `json:"clients,omitempty"`
}

func registerPopular(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "popular",
		Description: "Return popular tags combined across the requested clients. Rankings are fused with reciprocal " +
			"rank fusion, and each tag carries every contributing client's own count, rank, and within-query " +
			"percentage; raw counts are never summed across clients, because they come from different corpora. The " +
			"snapshot is cached with a long TTL.",
		InputSchema: popularSchema(),
	}, h.popular)
}

func (h *handlers) popular(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in popularInput,
) (*mcp.CallToolResult, popularOutput, error) {
	category, err := h.parseCategory(in.Category)
	if err != nil {
		return nil, popularOutput{}, fmt.Errorf("popular: %w", err)
	}

	h.log.Debug("tool called",
		zap.String("tool", "popular"),
		zap.Strings("clients", in.Clients),
		zap.String("category", in.Category),
		zap.Int("limit", in.Limit),
		zap.Bool("refresh", in.Refresh),
	)

	result, err := h.catalog.Popular(ctx, catalog.PopularInput{
		Clients:  in.Clients,
		Category: category,
		Limit:    in.Limit,
		Refresh:  in.Refresh,
	})
	if err != nil {
		h.log.Error("popular failed", zap.Error(err))

		return nil, popularOutput{}, fmt.Errorf("popular: %w", err)
	}

	out := popularOutput{
		Tags:     toTagOutputs(result.Tags),
		Skipped:  toSkippedOutputs(result.Skipped),
		Warnings: result.Warnings,
		Clients:  toClientStatusOutputs(result.Clients),
	}

	text := present.Popular(result.Tags)

	if note := present.Skipped(result.Skipped); note != "" {
		text += "\n" + note
	}

	if note := present.ClientStatuses(result.Clients); note != "" {
		text += "\n" + note
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func popularSchema() *jsonschema.Schema {
	schema := schemaFor[popularInput]("popular")
	setCategoryEnum(schema)
	setDefault(schema, "limit", 25)
	setDefault(schema, "refresh", false)

	return schema
}
