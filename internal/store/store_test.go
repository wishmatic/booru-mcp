package store

import (
	"context"
	"errors"
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

func TestNewUsesWAL(t *testing.T) {
	client := newTestStore(t)

	if mode := client.JournalMode(); mode != "wal" {
		t.Fatalf("JournalMode() = %q, want wal", mode)
	}
}

func TestNewFallsBackWhenWALUnavailable(t *testing.T) {
	apply := func(mode string) (string, error) {
		if mode == journalModeWAL {
			return "", errors.New("unable to open database file (14)")
		}

		return mode, nil
	}

	mode, err := chooseJournalMode(apply)
	if err != nil {
		t.Fatalf("chooseJournalMode() error: %v", err)
	}

	if mode != journalModeDelete {
		t.Fatalf("chooseJournalMode() = %q, want %q", mode, journalModeDelete)
	}
}

func TestChooseJournalModeFailsWhenNoModeApplies(t *testing.T) {
	apply := func(string) (string, error) {
		return "", errors.New("attempt to write a readonly database (1544)")
	}

	if _, err := chooseJournalMode(apply); err == nil {
		t.Fatal("chooseJournalMode() error = nil, want an error when no journal mode can be enabled")
	}
}

func TestNewRejectsUnwritablePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}

	path := filepath.Join(t.TempDir(), "test.db")

	if err := os.WriteFile(path, nil, 0o444); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if _, err := New(path); err == nil {
		t.Fatal("New() error = nil, want an error for an unwritable database file")
	}
}

func TestNewRejectsDirectoryPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	if _, err := New(path); err == nil {
		t.Fatal("New() error = nil, want an error for a directory path")
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
