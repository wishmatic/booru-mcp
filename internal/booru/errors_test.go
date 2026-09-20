package booru

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

func TestSearchTagsErrorSurfacesDanbooruMessage(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"error":"Error","message":"Invalid search: nope"}`))
	})

	_, err := client.SearchTags(context.Background(), TagQuery{Search: "x", Limit: 5})
	if err == nil {
		t.Fatal("SearchTags() error = nil, want an error")
	}

	if err.Error() != "danbooru: Invalid search: nope" {
		t.Errorf("error = %q, want just the message", err.Error())
	}
}

func TestSearchTagsKeepsUntranslatableFailures(t *testing.T) {
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>forbidden</html>"))
	})

	_, err := client.SearchTags(context.Background(), TagQuery{Search: "blue_eyes", Limit: 5})
	if err == nil {
		t.Fatal("SearchTags() error = nil, want an error")
	}

	var httpErr *fetch.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %v, want the *HTTPError preserved", err)
	}

	if strings.Contains(err.Error(), "message") {
		t.Errorf("error = %q, want no fabricated message", err.Error())
	}
}
