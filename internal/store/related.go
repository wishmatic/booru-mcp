package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

func (c *Client) ReplaceRelated(ctx context.Context, client, name string, tags []booru.RelatedTag, at time.Time) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin related replace: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM related_tags WHERE client = ? AND name = ?`, client, name); err != nil {
		return fmt.Errorf("store: clear related for %s/%s: %w", client, name, err)
	}

	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO related_tags (client, name, related, score, "rank", fetched_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("store: prepare related insert: %w", err)
	}

	defer statement.Close()

	for i, tag := range tags {
		if _, err := statement.ExecContext(ctx, client, name, tag.Tag, tag.Score, i+1, at.Unix()); err != nil {
			return fmt.Errorf("store: insert related %s/%s: %w", client, name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit related replace: %w", err)
	}

	return nil
}

func (c *Client) Related(ctx context.Context, client, name string) ([]booru.RelatedTag, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT related, score, "rank" FROM related_tags WHERE client = ? AND name = ? ORDER BY "rank" ASC`,
		client, name,
	)
	if err != nil {
		return nil, fmt.Errorf("store: query related: %w", err)
	}

	defer rows.Close()

	var out []booru.RelatedTag

	for rows.Next() {
		tag := booru.RelatedTag{Client: client}

		if err := rows.Scan(&tag.Tag, &tag.Score, &tag.Rank); err != nil {
			return nil, fmt.Errorf("store: scan related: %w", err)
		}

		out = append(out, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read related: %w", err)
	}

	return out, nil
}

func (c *Client) RelatedFetchedAt(ctx context.Context, client, name string) (time.Time, bool, error) {
	var fetchedAt sql.NullInt64

	err := c.db.QueryRowContext(ctx,
		`SELECT MAX(fetched_at) FROM related_tags WHERE client = ? AND name = ?`,
		client, name,
	).Scan(&fetchedAt)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("store: read related timestamp: %w", err)
	}

	if !fetchedAt.Valid || fetchedAt.Int64 == 0 {
		return time.Time{}, false, nil
	}

	return time.Unix(fetchedAt.Int64, 0).UTC(), true, nil
}
