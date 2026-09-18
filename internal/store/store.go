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

type Client struct {
	db *sql.DB
}

func New(path string) (*Client, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create directory for %s: %w", path, err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("store: enable WAL on %s (parent directory must exist and be writable): %w", path, err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("store: create schema: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}
