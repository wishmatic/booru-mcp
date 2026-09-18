package mcp

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/present"
	"go.uber.org/zap"
)

type getInput struct {
	ID     string `json:"id" jsonschema:"the post id to fetch"`
	Client string `json:"client,omitempty" jsonschema:"optional: the client to fetch from; defaults to danbooru"`
}

type getOutput struct {
	Post postOutput `json:"post"`
}

func registerGet(srv *mcp.Server, h *handlers) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get",
		Description: "Fetch one post with everything on it: every tag grouped by category, the permalink, source, " +
			"score, favourite count, dimensions, rating, and the file, sample, and preview URLs. A post id only " +
			"exists on one site, so this reads a single client and defaults to danbooru. No image bytes are " +
			"returned and nothing here is cached.",
		InputSchema: getSchema(),
	}, h.get)
}

func (h *handlers) get(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in getInput,
) (*mcp.CallToolResult, getOutput, error) {
	h.log.Debug("tool called",
		zap.String("tool", "get"),
		zap.String("id", in.ID),
		zap.String("client", in.Client),
	)

	post, err := h.catalog.Get(ctx, in.Client, in.ID)
	if err != nil {
		h.log.Error("get failed", zap.String("id", in.ID), zap.Error(err))

		return nil, getOutput{}, fmt.Errorf("get: %w", err)
	}

	out := getOutput{Post: toPostOutput(post, true)}
	text := present.Post(post)

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, out, nil
}

func getSchema() *jsonschema.Schema {
	return schemaFor[getInput]("get")
}
