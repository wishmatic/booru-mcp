package booru

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

func TestSearchTagsOnlyAuthenticatesWhenBothAreSet(t *testing.T) {
	tests := map[string]struct {
		login, key         string
		wantLogin, wantKey bool
	}{
		"both":       {login: "me", key: "secret", wantLogin: true, wantKey: true},
		"login only": {login: "me"},
		"key only":   {key: "secret"},
		"neither":    {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			log := &requestLog{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.record(r.URL.Query())
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(server.Close)

			httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, Limiter: noopLimiter{}})
			if err != nil {
				t.Fatalf("fetch.New() error: %v", err)
			}

			client := New(Config{BaseURL: server.URL, Login: tt.login, APIKey: tt.key, MaxLimit: 10, HTTP: httpClient})

			if _, err := client.SearchTags(context.Background(), TagQuery{Search: "x", Limit: 1}); err != nil {
				t.Fatalf("SearchTags() error: %v", err)
			}

			query := log.at(0)

			if (query.Get("login") != "") != tt.wantLogin || (query.Get("api_key") != "") != tt.wantKey {
				t.Errorf("login=%q api_key=%q, want login=%v key=%v",
					query.Get("login"), query.Get("api_key"), tt.wantLogin, tt.wantKey)
			}
		})
	}
}

func TestSearchTagsTypedBodyError(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	})

	_, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Limit: 5})
	if err == nil {
		t.Fatal("SearchTags() error = nil, want an error")
	}

	var bodyErr *fetch.BodyError
	if !errors.As(err, &bodyErr) {
		t.Fatalf("error = %v, want a typed *fetch.BodyError", err)
	}

	if !strings.Contains(err.Error(), "danbooru") || !strings.Contains(err.Error(), "/tags.json") ||
		!strings.Contains(err.Error(), "non-JSON") {
		t.Errorf("error = %q, want it to name the client, path, and non-JSON body", err)
	}
}
