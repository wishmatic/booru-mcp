package booru

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

type noopLimiter struct{}

func (noopLimiter) Wait(context.Context) error { return nil }

type requestLog struct {
	mu    sync.Mutex
	calls []url.Values
}

func (l *requestLog) record(query url.Values) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls = append(l.calls, query)
}

func (l *requestLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.calls)
}

func (l *requestLog) at(index int) url.Values {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index >= len(l.calls) {
		return url.Values{}
	}

	return l.calls[index]
}

func newTestClient(t *testing.T, maxLimit int, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient, err := fetch.New(fetch.Config{BaseURL: server.URL, UserAgent: "test", Limiter: noopLimiter{}})
	if err != nil {
		t.Fatalf("fetch.New() error: %v", err)
	}

	return New(Config{BaseURL: server.URL, MaxLimit: maxLimit, HTTP: httpClient})
}

func tagNames(count int) []string {
	names := make([]string, 0, count)

	for i := range count {
		names = append(names, fmt.Sprintf("blue_tag_%02d", i))
	}

	return names
}

// serveTagPage answers a tag listing request the way Danbooru does: the requested page of the requested size, with
// counts falling as the index rises and an empty page once the names run out.
func serveTagPage(w http.ResponseWriter, r *http.Request, names []string) {
	page := intParam(r, "page", 1)
	limit := intParam(r, "limit", 20)

	start := min((page-1)*limit, len(names))
	end := min(start+limit, len(names))

	items := make([]string, 0, end-start)

	for i := start; i < end; i++ {
		items = append(items, fmt.Sprintf(`{"name":%q,"category":0,"post_count":%d}`, names[i], 100000-i))
	}

	_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
}

func intParam(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value < 1 {
		return fallback
	}

	return value
}

func TestSearchTagsRequestShape(t *testing.T) {
	log := &requestLog{}

	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Query())
		serveTagPage(w, r, tagNames(30))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue_hair", Limit: 25})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	query := log.at(0)

	if got := query.Get("search[name_matches]"); got != "*blue_hair*" {
		t.Errorf("name_matches = %q, want *blue_hair*", got)
	}

	if got := query.Get("search[order]"); got != "count" {
		t.Errorf("search[order] = %q, want count", got)
	}

	if got := query.Get("limit"); got != "100" {
		t.Errorf("limit = %q, want the page size", got)
	}

	if got := query.Get("page"); got != "" {
		t.Errorf("page = %q, want it absent on the first page", got)
	}

	if len(page.Tags) != 25 || page.Tags[0].Name != "blue_tag_00" || page.Tags[0].Count != 100000 {
		t.Errorf("tags = %+v, want the first 25", page.Tags)
	}

	if !page.More {
		t.Error("More = false, want true with matches past the window")
	}
}

func TestSearchTagsWindowSpansPages(t *testing.T) {
	log := &requestLog{}
	names := tagNames(80)

	client := newTestClient(t, 25, func(w http.ResponseWriter, r *http.Request) {
		log.record(r.URL.Query())
		serveTagPage(w, r, names)
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Offset: 30, Limit: 25})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	want := names[30:55]
	if len(page.Tags) != len(want) {
		t.Fatalf("tags = %d, want %d", len(page.Tags), len(want))
	}

	for i, name := range want {
		if page.Tags[i].Name != name {
			t.Fatalf("tags[%d] = %q, want %q", i, page.Tags[i].Name, name)
		}
	}

	if log.count() != 2 {
		t.Errorf("requests = %d, want 2", log.count())
	}

	if got := log.at(0).Get("page"); got != "2" {
		t.Errorf("first page = %q, want 2", got)
	}

	if got := log.at(1).Get("page"); got != "3" {
		t.Errorf("second page = %q, want 3", got)
	}
}

func TestSearchTagsMoreAtTheBoundary(t *testing.T) {
	tests := map[string]struct {
		total int
		want  bool
	}{
		"matches continue past the window": {total: 30, want: true},
		"window ends at the last match":    {total: 25, want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := newTestClient(t, 25, func(w http.ResponseWriter, r *http.Request) {
				serveTagPage(w, r, tagNames(tt.total))
			})

			page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Limit: 25})
			if err != nil {
				t.Fatalf("SearchTags() error: %v", err)
			}

			if len(page.Tags) != min(tt.total, 25) {
				t.Fatalf("tags = %d, want %d", len(page.Tags), min(tt.total, 25))
			}

			if page.More != tt.want {
				t.Errorf("More = %v, want %v", page.More, tt.want)
			}
		})
	}
}

func TestSearchTagsPastTheEnd(t *testing.T) {
	client := newTestClient(t, 25, func(w http.ResponseWriter, r *http.Request) {
		serveTagPage(w, r, tagNames(5))
	})

	page, err := client.SearchTags(context.Background(), TagQuery{Search: "blue", Offset: 25, Limit: 25})
	if err != nil {
		t.Fatalf("SearchTags() error: %v", err)
	}

	if len(page.Tags) != 0 || page.More {
		t.Errorf("page = %+v, want an empty final window", page)
	}
}

func TestSearchTagsRejectsEmptySearch(t *testing.T) {
	client := newTestClient(t, 25, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the client made a request for an empty search")
	})

	if _, err := client.SearchTags(context.Background(), TagQuery{Search: "  ", Limit: 25}); err == nil {
		t.Fatal("SearchTags() error = nil, want an error")
	}
}
