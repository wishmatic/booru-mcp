package mcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"github.com/wishmatic/booru-mcp/internal/store"
	"go.uber.org/zap"
)

func newTestCatalog(t *testing.T, provider booru.Provider, opts catalog.Options) *catalog.Service {
	t.Helper()

	registry := booru.NewRegistry()
	if err := registry.Register("danbooru", provider, ""); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	db, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if len(opts.DefaultClients) == 0 {
		opts.DefaultClients = []string{"danbooru"}
	}

	return catalog.New(registry, db, opts)
}

func connectSession(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestNewRequiresCatalog(t *testing.T) {
	if _, err := New(Deps{Log: zap.NewNop()}); err == nil {
		t.Fatal("New() error = nil, want an error without a catalog")
	}
}

func TestRegistersFiveTools(t *testing.T) {
	service := newTestCatalog(t, &stubProvider{}, catalog.Options{MaxLimit: 10})

	srv, err := New(Deps{
		Log:                   zap.NewNop(),
		Catalog:               service,
		RelatedClients:        []string{"danbooru"},
		RelatedDefaultClients: []string{"danbooru"},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	got := make(map[string]bool)

	for _, tool := range result.Tools {
		got[tool.Name] = true
	}

	for _, want := range []string{"popular", "tags", "related", "search", "get"} {
		if !got[want] {
			t.Errorf("tool %q is not registered; got %v", want, result.Tools)
		}
	}

	if len(result.Tools) != 5 {
		t.Errorf("tools = %d, want 5", len(result.Tools))
	}
}

func TestRelatedSchemaRestrictsClients(t *testing.T) {
	service := newTestCatalog(t, &stubProvider{}, catalog.Options{MaxLimit: 10})

	srv, err := New(Deps{
		Log:                   zap.NewNop(),
		Catalog:               service,
		RelatedClients:        []string{"danbooru"},
		RelatedDefaultClients: []string{"danbooru"},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := connectSession(t, srv).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}

	var related *mcp.Tool

	for _, tool := range result.Tools {
		if tool.Name == "related" {
			related = tool
		}
	}

	if related == nil {
		t.Fatal("related tool is not registered")
	}

	props, ok := related.InputSchema.(map[string]any)["properties"].(map[string]any)
	if !ok {
		t.Fatalf("input schema has no properties: %#v", related.InputSchema)
	}

	clients, ok := props["clients"].(map[string]any)
	if !ok {
		t.Fatalf("clients property missing: %#v", props)
	}

	items, ok := clients["items"].(map[string]any)
	if !ok {
		t.Fatalf("clients items missing: %#v", clients)
	}

	enum, ok := items["enum"].([]any)
	if !ok || len(enum) != 1 {
		t.Fatalf("clients enum = %#v, want the related-capable clients only", items["enum"])
	}

	if defaults, ok := clients["default"].([]any); !ok || len(defaults) != 1 {
		t.Errorf("clients default = %#v, want the capable clients intersected with the default list", clients["default"])
	}
}
