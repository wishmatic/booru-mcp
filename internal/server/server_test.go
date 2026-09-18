package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/auth"
	"github.com/wishmatic/booru-mcp/internal/config"
	"go.uber.org/zap"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()

	return config.Config{
		Host:   "127.0.0.1",
		Port:   8080,
		APIKey: "server-key",
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	srv, err := New(testConfig(t), zap.NewNop())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return srv
}

func TestNewRequiresAPIKey(t *testing.T) {
	cfg := testConfig(t)
	cfg.APIKey = ""

	if _, err := New(cfg, zap.NewNop()); !errors.Is(err, auth.ErrNoAPIKey) {
		t.Fatalf("New() error = %v, want %v", err, auth.ErrNoAPIKey)
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestMCPRequiresAuth(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "no header"},
		{name: "wrong token", header: "Bearer wrong-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			rec := httptest.NewRecorder()
			newTestServer(t).router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	if err := newTestServer(t).Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error: %v", err)
	}
}
