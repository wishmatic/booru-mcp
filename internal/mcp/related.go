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

type relatedInput struct {
	clientsInput

	Tag     string `json:"tag" jsonschema:"the tag to find related tags for"`
	Limit   int    `json:"limit,omitempty" jsonschema:"optional: maximum number of tags to return; defaults to 25"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"optional: re-fetch from the clients even if the cache is fresh"`
}

type relatedTagOutput struct {
	Tag    string  `json:"tag"`
	Client string  `json:"client"`
	Score  float64 `json:"score"`
}

type relatedOutput struct {
	Tags     []relatedTagOutput   `json:"tags"`
	Skipped  []skippedOutput      `json:"skipped,omitempty"`
	Warnings []string             `json:"warnings,omitempty"`
	Clients  []clientStatusOutput `json:"clients,omitempty"`
}

func registerRelated(srv *mcp.Server, h *handlers, capable, defaults []string) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "related",
		Description: "Return tags that co-occur with a given tag. Related tags are only available from clients that " +
			"publish a related-tag API, so the `clients` list is limited to those and every other client is skipped " +
			"and named in the result. A short result, or an empty one, is expected behaviour rather than a failure: " +
			"do not retry it, and do not read an empty result as \"" + "this tag has no relatives" + "\". Rows from " +
			"different clients are kept separate rather than fused.",
		InputSchema: relatedSchema(capable, defaults),
	}, h.related)
}

func (h *handlers) related(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in relatedInput,
) (*mcp.CallToolResult, relatedOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "related"),
		zap.String("tag", in.Tag),
		zap.Strings("clients", in.Clients),
		zap.Int("limit", in.Limit),
		zap.Bool("refresh", in.Refresh),
	)

	result, err := h.catalog.Related(ctx, catalog.RelatedInput{
		Tag:     in.Tag,
		Clients: in.Clients,
		Limit:   in.Limit,
		Refresh: in.Refresh,
	})
	if err != nil {
		h.log.Error("related failed", zap.Error(err))

		return nil, relatedOutput{}, fmt.Errorf("related: %w", err)
	}

	out := relatedOutput{
		Skipped:  toSkippedOutputs(result.Skipped),
		Warnings: result.Warnings,
		Clients:  toClientStatusOutputs(result.Clients),
	}

	for _, tag := range result.Tags {
		out.Tags = append(out.Tags, relatedTagOutput{Tag: tag.Tag, Client: tag.Client, Score: tag.Score})
	}

	text := present.Related(result.Tags, result.Skipped)

	if note := present.ClientStatuses(result.Clients); note != "" {
		text += "\n" + note
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func relatedSchema(capable, defaults []string) *jsonschema.Schema {
	schema := schemaFor[relatedInput]("related")

	schema.Properties["clients"].Items.Enum = stringEnum(capable)
	setDefault(schema, "clients", defaults)
	setDefault(schema, "limit", 25)
	setDefault(schema, "refresh", false)

	return schema
}

func stringEnum(values []string) []any {
	enum := make([]any, 0, len(values))
	for _, value := range values {
		enum = append(enum, value)
	}

	return enum
}
