package store

import (
	"context"
	"fmt"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func (c *Client) ReplacePopular(ctx context.Context, client string, tags []booru.Tag, at time.Time) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin popular replace: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM popular_tags WHERE client = ?`, client); err != nil {
		return fmt.Errorf("store: clear popular for %s: %w", client, err)
	}

	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO popular_tags (client, name, "rank", "count", category, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store: prepare popular insert: %w", err)
	}

	defer statement.Close()

	for i, tag := range tags {
		if _, err := statement.ExecContext(ctx, client, tag.Name, i+1, tag.Count, tag.Category.String(), at.Unix()); err != nil {
			return fmt.Errorf("store: insert popular %s/%s: %w", client, tag.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit popular replace: %w", err)
	}

	return nil
}

func (c *Client) Popular(ctx context.Context, client string) ([]CachedTag, error) {
	query := `SELECT client, name, category, "count", fetched_at, 1 FROM popular_tags
		WHERE client = ? ORDER BY "rank" ASC`

	return c.queryCachedTags(ctx, query, client)
}

func (c *Client) IsPopular(ctx context.Context, client, name string) (bool, error) {
	var exists bool

	err := c.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM popular_tags WHERE client = ? AND name = ?)`,
		client, name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("store: check popular %s/%s: %w", client, name, err)
	}

	return exists, nil
}
