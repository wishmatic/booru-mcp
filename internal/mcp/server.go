package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

func New() *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{
		Name:    "booru-mcp",
		Version: "0.1.0",
	}, nil)
}
