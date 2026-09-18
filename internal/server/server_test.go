package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/wishmatic/booru-mcp/internal/auth"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"github.com/wishmatic/booru-mcp/internal/config"
	"github.com/wishmatic/booru-mcp/internal/danbooru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
	"github.com/wishmatic/booru-mcp/internal/gelbooru"
	mcpServer "github.com/wishmatic/booru-mcp/internal/mcp"
	"github.com/wishmatic/booru-mcp/internal/store"
)

type noopLimiter struct{}

func (noopLimiter) Wait(context.Context) error { return nil }

func testConfig(t *testing.T) config.Config {
	t.Helper()

	for _, key := range []string{
		"KONACHAN_URL", "DANBOORU_LOGIN", "DANBOORU_API_KEY",
		"RULE34_API_KEY", "RULE34_USER_ID", "E621_LOGIN", "E621_API_KEY",
	} {
		t.Setenv(key, "")
	}

	return config.Config{
		Host:                  "127.0.0.1",
		Port:                  8080,
		APIKey:                "server-key",
		Clients:               "danbooru",
		DefaultClientsRaw:     "danbooru",
		UserAgent:             config.DefaultUserAgent,
		RateLimitRPS:          1,
		RateLimitBurst:        1,
		RequestTimeoutSeconds: 20,
		MaxLimit:              100,
		CacheTTLDays:          30,
		DBPath:                filepath.Join(t.TempDir(), "test.db"),
		ContentRatingRaw:      "all",
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	cfg := testConfig(t)
	cfg.APIKey = ""

	if _, err := New(cfg, zap.NewNop()); !errors.Is(err, auth.ErrNoAPIKey) {
		t.Fatalf("New() error = %v, want %v", err, auth.ErrNoAPIKey)
	}
}

func TestNewValidationNamesEnvar(t *testing.T) {
	cfg := testConfig(t)
	cfg.RateLimitRPS = 0

	if _, err := New(cfg, zap.NewNop()); err == nil || !strings.Contains(err.Error(), "RATE_LIMIT_RPS") {
		t.Fatalf("New() error = %v, want it to name RATE_LIMIT_RPS", err)
	}
}

func TestInactiveClientLogsReason(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)
	cfg.Clients = "danbooru,rule34"

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if _, err := srv.registry.Get("rule34"); err != nil {
		t.Fatalf("Get(rule34) error: %v", err)
	}

	found := false

	for _, entry := range logs.All() {
		if entry.Message == "client inactive" && entry.ContextMap()["client"] == "rule34" {
			found = true
		}
	}

	if !found {
		t.Error("no inactive-client log entry for rule34")
	}
}

func TestLogsDoNotLeakCredentials(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)
	cfg.Clients = "danbooru,rule34"

	t.Setenv("RULE34_API_KEY", "super-secret-value")
	t.Setenv("RULE34_USER_ID", "")

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "super-secret-value") {
			t.Fatalf("log entry %q leaks a credential", entry.Message)
		}
	}
}

func TestHealthzAndAuth(t *testing.T) {
	cfg := testConfig(t)

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("healthz = %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("mcp without auth = %d, want 401", rec.Code)
	}
}

func TestShutdownClosesStore(t *testing.T) {
	cfg := testConfig(t)

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}
}

func TestAllClientsRegisterWithReasons(t *testing.T) {
	cfg := testConfig(t)
	cfg.Clients = "all"

	srv, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	if got := len(srv.registry.All()); got != len(config.ClientSpecs()) {
		t.Fatalf("registered clients = %d, want %d", got, len(config.ClientSpecs()))
	}

	reasons := make(map[string]bool)

	for _, entry := range srv.registry.All() {
		if entry.Active {
			continue
		}

		if entry.Reason == "" {
			t.Errorf("inactive client %s has no reason", entry.Name)
		}

		reasons[entry.Reason] = true
	}

	if len(reasons) < 2 {
		t.Errorf("inactive reasons = %d distinct, want each unmet requirement to read differently", len(reasons))
	}
}

func TestNoActiveClientsWarnsAndStarts(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	cfg := testConfig(t)
	cfg.Clients = "rule34"
	cfg.DefaultClientsRaw = "rule34"

	srv, err := New(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("New() error: %v, want startup to succeed with no active clients", err)
	}

	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	warned := false

	for _, entry := range logs.All() {
		if entry.Level == zapcore.WarnLevel && strings.Contains(entry.Message, "no clients are active") {
			warned = true
		}
	}

	if !warned {
		t.Error("no warning about every client being inactive")
	}
}

func TestCombinedPopularThroughHandler(t *testing.T) {
	danbooruUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"blue_eyes","category":0,"post_count":900}]`))
	}))
	t.Cleanup(danbooruUpstream.Close)

	gelbooruUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag":[{"name":"red hair","count":500,"type":0}]}`))
	}))
	t.Cleanup(gelbooruUpstream.Close)

	danbooruTransport, err := fetch.New(fetch.Config{BaseURL: danbooruUpstream.URL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	gelbooruTransport, err := fetch.New(fetch.Config{BaseURL: gelbooruUpstream.URL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	registry := booru.NewRegistry()

	if err := registry.Register("danbooru", danbooru.New(danbooru.Config{
		BaseURL: danbooruUpstream.URL, MaxLimit: 100, HTTP: danbooruTransport,
	}), ""); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if err := registry.Register("rule34", gelbooru.New(gelbooru.Config{
		Name: "rule34", BaseURL: gelbooruUpstream.URL, MaxLimit: 100, HTTP: gelbooruTransport,
	}), ""); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	db, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.New() error: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	service := catalog.New(registry, db, catalog.Options{
		DefaultClients: []string{"danbooru", "rule34"},
		CacheTTL:       time.Hour,
		ContentRating:  booru.RatingAll,
		MaxLimit:       100,
	})

	srv, err := mcpServer.New(mcpServer.Deps{Log: zap.NewNop(), Catalog: service})
	if err != nil {
		t.Fatalf("mcp.New() error: %v", err)
	}

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

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "popular"})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("popular returned an error: %+v", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want text", result.Content[0])
	}

	for _, want := range []string{"blue_eyes", "red_hair"} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("popular text %q does not contain %q", text.Text, want)
		}
	}
}
