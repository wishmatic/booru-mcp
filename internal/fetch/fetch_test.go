package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type countingLimiter struct {
	mu    sync.Mutex
	waits int
	err   error
}

func (l *countingLimiter) Wait(context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.waits++

	return l.err
}

func (l *countingLimiter) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.waits
}

func newTestClient(t *testing.T, baseURL string, limiter Limiter) *Client {
	t.Helper()

	c, err := New(Config{
		BaseURL:   baseURL,
		UserAgent: "booru-mcp-test",
		Timeout:   5 * time.Second,
		Limiter:   limiter,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return c
}

func TestRequestHeaders(t *testing.T) {
	var got http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	if _, err := GetJSON[map[string]bool](context.Background(), newTestClient(t, server.URL, &countingLimiter{}), "danbooru", "/posts.json", nil); err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}

	if ua := got.Get("User-Agent"); ua != "booru-mcp-test" {
		t.Errorf("User-Agent = %q, want booru-mcp-test", ua)
	}

	if accept := got.Get("Accept"); accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", accept)
	}
}

func TestQueryIsEncoded(t *testing.T) {
	var gotQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	query := url.Values{"tags": {"blue_eyes"}, "limit": {"3"}}
	if _, err := GetJSON[map[string]any](context.Background(), newTestClient(t, server.URL, &countingLimiter{}), "danbooru", "/posts.json", query); err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}

	if got := gotQuery.Get("tags"); got != "blue_eyes" {
		t.Errorf("tags = %q, want blue_eyes", got)
	}

	if got := gotQuery.Get("limit"); got != "3" {
		t.Errorf("limit = %q, want 3", got)
	}
}

func TestRetryThenSucceed(t *testing.T) {
	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("slow down"))

			return
		}

		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	limiter := &countingLimiter{}

	if _, err := GetJSON[map[string]bool](context.Background(), newTestClient(t, server.URL, limiter), "danbooru", "/posts.json", nil); err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}

	if limiter.count() != 2 {
		t.Errorf("limiter waits = %d, want 2", limiter.count())
	}
}

func TestRetriesExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("boom"))
	}))
	t.Cleanup(server.Close)

	limiter := &countingLimiter{}

	_, err := GetJSON[map[string]any](context.Background(), newTestClient(t, server.URL, limiter), "danbooru", "/posts.json", nil)
	if err == nil {
		t.Fatal("GetJSON() error = nil, want an error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %v, want an *HTTPError", err)
	}

	if httpErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want 503", httpErr.StatusCode)
	}

	if limiter.count() != maxAttempts {
		t.Errorf("limiter waits = %d, want %d", limiter.count(), maxAttempts)
	}

	for _, want := range []string{"danbooru", "/posts.json", "503", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestErrorBodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("a", 10_000)))
	}))
	t.Cleanup(server.Close)

	_, err := GetJSON[map[string]any](context.Background(), newTestClient(t, server.URL, &countingLimiter{}), "danbooru", "/x", nil)
	if err == nil {
		t.Fatal("GetJSON() error = nil, want an error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %v, want an *HTTPError", err)
	}

	if len(httpErr.Body) >= 10_000 {
		t.Errorf("body length = %d, want it bounded", len(httpErr.Body))
	}
}

func TestNonRetryableStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	limiter := &countingLimiter{}

	_, err := GetJSON[map[string]any](context.Background(), newTestClient(t, server.URL, limiter), "danbooru", "/posts.json", nil)
	if err == nil {
		t.Fatal("GetJSON() error = nil, want an error")
	}

	if limiter.count() != 1 {
		t.Errorf("limiter waits = %d, want 1 for a non-retryable status", limiter.count())
	}
}

func TestContextDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := GetJSON[map[string]any](ctx, newTestClient(t, server.URL, &countingLimiter{}), "danbooru", "/posts.json", nil)
	if err == nil {
		t.Fatal("GetJSON() error = nil, want an error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	cases := map[string]Config{
		"relative":    {BaseURL: "example.com/path", Limiter: &countingLimiter{}},
		"file":        {BaseURL: "file:///x", Limiter: &countingLimiter{}},
		"nil limiter": {BaseURL: "https://example.com"},
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New(cfg); err == nil {
				t.Fatalf("New(%+v) error = nil, want an error", cfg)
			}
		})
	}

	if _, err := New(Config{BaseURL: "https://example.com", Limiter: &countingLimiter{}}); err != nil {
		t.Fatalf("New() error: %v", err)
	}
}

func TestNewLimiterDefaults(t *testing.T) {
	if l := NewLimiter(0, 0); l.Limit() != 1 {
		t.Errorf("Limit() = %v, want 1", l.Limit())
	}

	if l := NewLimiter(5, 2); l.Limit() != 5 || l.Burst() != 2 {
		t.Errorf("Limit() = %v, Burst() = %d, want 5 and 2", l.Limit(), l.Burst())
	}
}
