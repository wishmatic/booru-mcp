package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func newTestStore(t *testing.T) *Client {
	t.Helper()

	client, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestNewCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "test.db")

	client, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
}

func TestSchemaCreatedAndReopens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	at := time.Unix(1_700_000_000, 0).UTC()

	if err := first.UpsertTags(context.Background(), "danbooru", []booru.Tag{
		{Name: "blue_eyes", Category: booru.CategoryGeneral, Count: 100},
	}, at); err != nil {
		t.Fatalf("UpsertTags() error: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	second, err := New(path)
	if err != nil {
		t.Fatalf("New() second error: %v", err)
	}

	t.Cleanup(func() { _ = second.Close() })

	tags, err := second.TagsByClient(context.Background(), "danbooru", []string{"blue_eyes"})
	if err != nil {
		t.Fatalf("TagsByClient() error: %v", err)
	}

	if len(tags) != 1 || tags[0].Tag.Count != 100 {
		t.Fatalf("TagsByClient() = %+v, want the persisted tag", tags)
	}
}
