package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
)

type CachedTag struct {
	Client    string
	Tag       booru.Tag
	FetchedAt time.Time
	IsPopular bool
}

type TagFilter struct {
	Client   string
	Query    string
	Category booru.TagCategory
	Limit    int
}

const cachedTagColumns = `client, name, category, "count", fetched_at,
	EXISTS(SELECT 1 FROM popular_tags p WHERE p.client = tags.client AND p.name = tags.name)`

func (c *Client) UpsertTags(ctx context.Context, client string, tags []booru.Tag, at time.Time) error {
	if len(tags) == 0 {
		return nil
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin upsert: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO tags (client, name, category, "count", fetched_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (client, name) DO UPDATE SET
			category = excluded.category,
			"count" = excluded."count",
			fetched_at = excluded.fetched_at`)
	if err != nil {
		return fmt.Errorf("store: prepare upsert: %w", err)
	}

	defer statement.Close()

	for _, tag := range tags {
		if _, err := statement.ExecContext(ctx, client, tag.Name, tag.Category.String(), tag.Count, at.Unix()); err != nil {
			return fmt.Errorf("store: upsert %s/%s: %w", client, tag.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit upsert: %w", err)
	}

	return nil
}

func (c *Client) TagsByClient(ctx context.Context, client string, names []string) ([]CachedTag, error) {
	if len(names) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(names))
	args := make([]any, 0, len(names)+1)
	args = append(args, client)

	for i, name := range names {
		placeholders[i] = "?"
		args = append(args, name)
	}

	query := `SELECT ` + cachedTagColumns + ` FROM tags WHERE client = ? AND name IN (` +
		strings.Join(placeholders, ", ") + `) ORDER BY "count" DESC, name ASC`

	return c.queryCachedTags(ctx, query, args...)
}

func (c *Client) SearchTags(ctx context.Context, filter TagFilter) ([]CachedTag, error) {
	query := `SELECT ` + cachedTagColumns + ` FROM tags WHERE client = ?`
	args := []any{filter.Client}

	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, filter.Category.String())
	}

	if filter.Query != "" {
		query += ` AND name LIKE ? ESCAPE '\'`
		args = append(args, likePattern(filter.Query))
	}

	query += ` ORDER BY "count" DESC, name ASC`

	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}

	return c.queryCachedTags(ctx, query, args...)
}

func (c *Client) queryCachedTags(ctx context.Context, query string, args ...any) ([]CachedTag, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query tags: %w", err)
	}

	defer rows.Close()

	var out []CachedTag

	for rows.Next() {
		var (
			cached    CachedTag
			category  string
			fetchedAt int64
		)

		if err := rows.Scan(
			&cached.Client,
			&cached.Tag.Name,
			&category,
			&cached.Tag.Count,
			&fetchedAt,
			&cached.IsPopular,
		); err != nil {
			return nil, fmt.Errorf("store: scan tag: %w", err)
		}

		cached.Tag.Category = booru.TagCategory(category)
		cached.FetchedAt = time.Unix(fetchedAt, 0).UTC()

		out = append(out, cached)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read tags: %w", err)
	}

	return out, nil
}

func likePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	return "%" + replacer.Replace(value) + "%"
}
