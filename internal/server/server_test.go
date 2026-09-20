package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/wishmatic/booru-mcp/internal/auth"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/config"
	"github.com/wishmatic/booru-mcp/internal/fetch"
)

type noopLimiter struct{}

func (noopLimiter) Wait(context.Context) error { return nil }

type authTransport struct {
	key  string
	base http.RoundTripper
}

func (t authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.key)

	return t.base.RoundTrip(clone)
}

func testConfig() config.Config {
	return config.Config{
		Host:                  "127.0.0.1",
		Port:                  8080,
		APIKey:                "server-key",
		UserAgent:             "booru-mcp/0.1.0",
		RateLimitRPS:          1,
		RateLimitBurst:        1,
		RequestTimeoutSeconds: 20,
		MaxLimit:              100,
		MaxOffset:             1000,
	}
}

func newTestServer(t *testing.T, cfg config.Config, upstreamURL string) *Server {
	t.Helper()

	transport, err := fetch.New(fetch.Config{BaseURL: upstreamURL, Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	provider := booru.New(booru.Config{BaseURL: upstreamURL, MaxLimit: cfg.MaxLimit, HTTP: transport})

	srv, err := newWithProvider(cfg, zap.NewNop(), provider)
	if err != nil {
		t.Fatalf("newWithProvider() error: %v", err)
	}

	return srv
}

func TestNewRequiresAPIKey(t *testing.T) {
	cfg := testConfig()
	cfg.APIKey = ""

	if _, err := New(cfg, zap.NewNop()); !errors.Is(err, auth.ErrNoAPIKey) {
		t.Fatalf("New() error = %v, want %v", err, auth.ErrNoAPIKey)
	}
}

func TestNewWithoutDanbooruCredentialsSucceeds(t *testing.T) {
	if _, err := New(testConfig(), zap.NewNop()); err != nil {
		t.Fatalf("New() error = %v, want Danbooru to work anonymously", err)
	}
}

func TestNewValidationNamesEnvar(t *testing.T) {
	for name, apply := range map[string]func(*config.Config){
		"RATE_LIMIT_RPS": func(c *config.Config) { c.RateLimitRPS = 0 },
		"MAX_LIMIT":      func(c *config.Config) { c.MaxLimit = 101 },
		"MAX_OFFSET":     func(c *config.Config) { c.MaxOffset = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := testConfig()
			apply(&cfg)

			if _, err := New(cfg, zap.NewNop()); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("New() error = %v, want it to name %s", err, name)
			}
		})
	}
}

func TestHealthzAndAuth(t *testing.T) {
	srv := newTestServer(t, testConfig(), "http://127.0.0.1:1")

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

func TestShutdown(t *testing.T) {
	srv := newTestServer(t, testConfig(), "http://127.0.0.1:1")

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}
}

func TestTagsCallMakesOneUpstreamRequest(t *testing.T) {
	var calls atomic.Int64

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`[{"name":"blue_hair","category":0,"post_count":482913}]`))
	}))
	t.Cleanup(upstream.Close)

	cfg := testConfig()
	srv := newTestServer(t, cfg, upstream.URL)

	httpServer := httptest.NewServer(srv.router)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)

	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: authTransport{key: cfg.APIKey, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "tags",
		Arguments: map[string]any{"search": "blue hair"},
	})
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tags returned an error: %+v", result.Content)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "blue_hair") {
		t.Fatalf("content = %#v, want the tag rendered", result.Content[0])
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want exactly one", got)
	}
}
