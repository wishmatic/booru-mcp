package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS tags (
	client     TEXT    NOT NULL,
	name       TEXT    NOT NULL,
	category   TEXT    NOT NULL,
	"count"    INTEGER NOT NULL,
	fetched_at INTEGER NOT NULL,
	PRIMARY KEY (client, name)
);

CREATE TABLE IF NOT EXISTS popular_tags (
	client     TEXT    NOT NULL,
	name       TEXT    NOT NULL,
	"rank"     INTEGER NOT NULL,
	"count"    INTEGER NOT NULL,
	category   TEXT    NOT NULL,
	fetched_at INTEGER NOT NULL,
	PRIMARY KEY (client, name)
);

CREATE TABLE IF NOT EXISTS related_tags (
	client     TEXT    NOT NULL,
	name       TEXT    NOT NULL,
	related    TEXT    NOT NULL,
	score      INTEGER NOT NULL,
	"rank"     INTEGER NOT NULL,
	fetched_at INTEGER NOT NULL,
	PRIMARY KEY (client, name, related)
);
`

const (
	journalModeWAL    = "wal"
	journalModeDelete = "delete"
)

type Client struct {
	db          *sql.DB
	journalMode string
}

func New(path string) (*Client, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create directory for %s: %w", path, err)
	}

	if err := ensureWritable(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	mode, err := configureJournal(db)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("store: configure %s: %w", path, err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("store: create schema: %w", err)
	}

	return &Client{db: db, journalMode: mode}, nil
}

func (c *Client) JournalMode() string {
	return c.journalMode
}

func (c *Client) Close() error {
	return c.db.Close()
}

func ensureWritable(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("store: open %s for read and write: %w", path, err)
	}

	return file.Close()
}

func configureJournal(db *sql.DB) (string, error) {
	return chooseJournalMode(func(mode string) (string, error) {
		return applyJournalMode(db, mode)
	})
}

func chooseJournalMode(apply func(mode string) (string, error)) (string, error) {
	mode, walErr := apply(journalModeWAL)
	if walErr == nil {
		return mode, nil
	}

	mode, err := apply(journalModeDelete)
	if err != nil {
		return "", fmt.Errorf("WAL unavailable (%v) and rollback journal failed: %w", walErr, err)
	}

	return mode, nil
}

func applyJournalMode(db *sql.DB, mode string) (string, error) {
	var applied string
	if err := db.QueryRow("PRAGMA journal_mode=" + mode).Scan(&applied); err != nil {
		return "", err
	}

	if applied != mode {
		return "", fmt.Errorf("sqlite applied journal_mode %s, want %s", applied, mode)
	}

	return applied, nil
}
