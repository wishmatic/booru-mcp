package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func statusByClient(t *testing.T, statuses []booru.ClientStatus, client string) booru.ClientStatus {
	t.Helper()

	for _, status := range statuses {
		if status.Client == client {
			return status
		}
	}

	t.Fatalf("no status for client %q in %+v", client, statuses)

	return booru.ClientStatus{}
}

func TestSearchReportsPerClientStatus(t *testing.T) {
	failing := &fakeProvider{name: "rule34", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return nil, errors.New("rule34 /index.php returned HTTP 500")
	}}
	empty := &fakeProvider{name: "xbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return nil, nil
	}}
	healthy := &fakeProvider{name: "danbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return []booru.Post{post("danbooru", "1", booru.RatingGeneral)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: 0},
		testEntry{name: "danbooru", provider: healthy},
		testEntry{name: "rule34", provider: failing},
		testEntry{name: "xbooru", provider: empty},
	)

	result, err := service.Search(context.Background(), SearchInput{
		Tags:    "x",
		Clients: []string{"danbooru", "rule34", "xbooru"},
	})
	if err != nil {
		t.Fatalf("Search() error: %v, want a partial failure to succeed", err)
	}

	if len(result.Posts) != 1 || len(result.Clients) != 3 {
		t.Fatalf("result = %+v, want one post from the healthy client and three statuses", result)
	}

	ok := statusByClient(t, result.Clients, "danbooru")
	if ok.State != booru.ClientStateOK || ok.Results != 1 {
		t.Errorf("danbooru status = %+v, want ok with one result", ok)
	}

	failed := statusByClient(t, result.Clients, "rule34")
	if failed.State != booru.ClientStateError || !strings.Contains(failed.Detail, "HTTP 500") {
		t.Errorf("rule34 status = %+v, want an error naming the failure", failed)
	}

	emptyStatus := statusByClient(t, result.Clients, "xbooru")
	if emptyStatus.State != booru.ClientStateOK || emptyStatus.Results != 0 {
		t.Errorf("xbooru status = %+v, want ok with zero results, distinct from an error", emptyStatus)
	}
}

func TestSearchReportsRandomNotApplied(t *testing.T) {
	plain := &fakeProvider{name: "gelbooru", searchFn: func(booru.SearchParams) ([]booru.Post, error) {
		return nil, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: 0},
		testEntry{name: "gelbooru", provider: plain})

	result, err := service.Search(context.Background(), SearchInput{Tags: "x", Random: true})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	status := statusByClient(t, result.Clients, "gelbooru")
	if status.State != booru.ClientStateOK || !strings.Contains(status.Detail, "random ordering not applied") {
		t.Errorf("gelbooru status = %+v, want the random limitation reported", status)
	}
}

func TestTagsReportsPerClientStatus(t *testing.T) {
	failing := &fakeProvider{name: "rule34", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return nil, errors.New("rule34: authentication required")
	}}
	healthy := &fakeProvider{name: "danbooru", tagsFn: func(booru.TagQuery) ([]booru.Tag, error) {
		return []booru.Tag{tag("blue_eyes", 100)}, nil
	}}

	service := newTestService(t, Options{MaxLimit: 10, CacheTTL: 0},
		testEntry{name: "danbooru", provider: healthy},
		testEntry{name: "rule34", provider: failing},
	)

	result, err := service.Tags(context.Background(), TagsInput{Query: "blue", Clients: []string{"danbooru", "rule34"}})
	if err != nil {
		t.Fatalf("Tags() error: %v", err)
	}

	if statusByClient(t, result.Clients, "rule34").State != booru.ClientStateError {
		t.Errorf("Clients = %+v, want rule34 reported as an error", result.Clients)
	}

	if statusByClient(t, result.Clients, "danbooru").Results != 1 {
		t.Errorf("Clients = %+v, want danbooru's result count", result.Clients)
	}
}
