package mcp

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"go.uber.org/zap"
)

type Deps struct {
	Log                   *zap.Logger
	Catalog               *catalog.Service
	RelatedClients        []string
	RelatedDefaultClients []string
}

func New(deps Deps) (*mcp.Server, error) {
	if deps.Catalog == nil {
		return nil, errors.New("mcp: catalog service is required")
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "booru-mcp",
		Version: "0.1.0",
	}, nil)

	h := &handlers{log: deps.Log, catalog: deps.Catalog}

	registerPopular(srv, h)
	registerTags(srv, h)
	registerRelated(srv, h, deps.RelatedClients, deps.RelatedDefaultClients)
	registerSearch(srv, h)
	registerGet(srv, h)

	return srv, nil
}
