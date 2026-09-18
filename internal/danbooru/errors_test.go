package danbooru

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
)

func tagLimitHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnprocessableEntity)
	_, _ = w.Write([]byte(`{"success":false,"error":"PostQuery::TagLimitError",` +
		`"message":"You cannot search for more than 2 tags at a time.",` +
		`"backtrace":["app/logical/post_query.rb:321:in 'PostQuery::ValidationMethods#validate_tag_limit!'"]}`))
}

func TestSearchTagLimitErrorNamesTheTermsSent(t *testing.T) {
	client := newTestProvider(t, tagLimitHandler)
	client.credential = []string{"DANBOORU_LOGIN", "DANBOORU_API_KEY"}

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "blue_eyes smile", Random: true})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	for _, want := range []string{
		"You cannot search for more than 2 tags at a time.",
		`terms sent: "blue_eyes smile order:random"`,
		"DANBOORU_LOGIN and DANBOORU_API_KEY raise the limit to 6 with a Gold account",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}

	if strings.Contains(err.Error(), "backtrace") {
		t.Errorf("error %q carries backtrace noise", err.Error())
	}
}

func TestSearchTagLimitErrorOmitsCredentialHintWhenAuthenticated(t *testing.T) {
	client := newTestProvider(t, tagLimitHandler)
	client.login = "someone"
	client.apiKey = "secret"
	client.credential = []string{"DANBOORU_LOGIN", "DANBOORU_API_KEY"}

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "blue_eyes smile"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	if strings.Contains(err.Error(), "DANBOORU_LOGIN") {
		t.Errorf("error %q hints at credentials that are already set", err.Error())
	}
}

func TestTagSearchErrorSurfacesDanbooruMessage(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"success":false,"error":"Error","message":"Invalid search: nope"}`))
	})

	_, err := client.SearchTags(context.Background(), booru.TagQuery{Query: "x"})
	if err == nil {
		t.Fatal("SearchTags() error = nil, want an error")
	}

	if err.Error() != "danbooru: Invalid search: nope" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestSearchErrorKeepsUntranslatableFailures(t *testing.T) {
	client := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>forbidden</html>"))
	})

	_, err := client.Search(context.Background(), booru.SearchParams{Tags: "blue_eyes"})
	if err == nil {
		t.Fatal("Search() error = nil, want an error")
	}

	var httpErr *fetch.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %v, want the *HTTPError preserved", err)
	}
}
