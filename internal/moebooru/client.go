package moebooru

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
)

type Config struct {
	Name     string
	BaseURL  string
	MaxLimit int
	HTTP     *fetch.Client
}

type Client struct {
	name     string
	baseURL  string
	maxLimit int
	http     *fetch.Client
}

func New(cfg Config) *Client {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "yandere"
	}

	maxLimit := cfg.MaxLimit
	if maxLimit < 1 {
		maxLimit = 100
	}

	return &Client{
		name:     name,
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		maxLimit: maxLimit,
		http:     cfg.HTTP,
	}
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Capabilities() booru.Capabilities {
	return booru.Capabilities{Rating: true}
}

func (c *Client) Search(ctx context.Context, params booru.SearchParams) ([]booru.Post, error) {
	values := url.Values{}
	values.Set("tags", searchTerms(params))
	values.Set("limit", strconv.Itoa(c.limit(params.Limit)))

	if params.Page > 1 {
		values.Set("page", strconv.Itoa(params.Page))
	}

	raw, err := fetch.GetJSON[[]postJSON](ctx, c.http, c.name, "/post.json", values)
	if err != nil {
		return nil, err
	}

	posts := make([]booru.Post, 0, len(raw))
	for _, item := range raw {
		posts = append(posts, c.toPost(item))
	}

	return posts, nil
}

func (c *Client) Post(ctx context.Context, id string) (booru.Post, error) {
	values := url.Values{"tags": {"id:" + id}}

	raw, err := fetch.GetJSON[[]postJSON](ctx, c.http, c.name, "/post.json", values)
	if err != nil {
		return booru.Post{}, err
	}

	if len(raw) == 0 {
		return booru.Post{}, fmt.Errorf("%s: post %s not found", c.name, id)
	}

	return c.toPost(raw[0]), nil
}

func (c *Client) SearchTags(ctx context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("order", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	if query.Query != "" {
		values.Set("name", wildcard(query.Query))
	}

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tag.json", values)
	if err != nil {
		return nil, err
	}

	return toTags(raw), nil
}

func (c *Client) PopularTags(ctx context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("order", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tag.json", values)
	if err != nil {
		return nil, err
	}

	return toTags(raw), nil
}

func (c *Client) limit(requested int) int {
	if requested <= 0 {
		return c.maxLimit
	}

	if requested > c.maxLimit {
		return c.maxLimit
	}

	return requested
}

func (c *Client) toPost(raw postJSON) booru.Post {
	id := strconv.Itoa(raw.ID)

	return booru.Post{
		Client:     c.name,
		ID:         id,
		URL:        fmt.Sprintf("%s/post/show/%s", c.baseURL, id),
		FileURL:    raw.FileURL,
		SampleURL:  raw.SampleURL,
		PreviewURL: raw.PreviewURL,
		Width:      raw.Width,
		Height:     raw.Height,
		Rating:     parseRating(raw.Rating),
		Score:      raw.Score,
		Source:     raw.Source,
		Tags:       postTags(raw.Tags),
		CreatedAt:  parseTime(raw.CreatedAt),
	}
}

func toTags(raw []tagJSON) []booru.Tag {
	tags := make([]booru.Tag, 0, len(raw))

	for _, item := range raw {
		tags = append(tags, booru.Tag{
			Name:     booru.NormalizeTag(item.Name),
			Category: tagCategory(item.Type),
			Count:    item.Count,
		})
	}

	return tags
}

func searchTerms(params booru.SearchParams) string {
	parts := make([]string, 0, len(params.Exclude)+2)

	if trimmed := strings.TrimSpace(params.Tags); trimmed != "" {
		parts = append(parts, trimmed)
	}

	if rating := ratingTerm(params.Rating); rating != "" {
		parts = append(parts, rating)
	}

	for _, exclude := range params.Exclude {
		if tag := booru.NormalizeTag(exclude); tag != "" {
			parts = append(parts, "-"+tag)
		}
	}

	return strings.Join(parts, " ")
}

func wildcard(value string) string {
	value = strings.TrimSpace(value)

	if strings.ContainsAny(value, "*?") {
		return value
	}

	return "*" + value + "*"
}

func postTags(value string) []booru.Tag {
	fields := strings.Fields(value)
	tags := make([]booru.Tag, 0, len(fields))

	for _, name := range fields {
		if normalized := booru.NormalizeTag(name); normalized != "" {
			tags = append(tags, booru.Tag{Name: normalized, Category: booru.CategoryGeneral})
		}
	}

	return tags
}

type postJSON struct {
	ID         int    `json:"id"`
	CreatedAt  int64  `json:"created_at"`
	Score      int    `json:"score"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Rating     string `json:"rating"`
	Source     string `json:"source"`
	FileURL    string `json:"file_url"`
	SampleURL  string `json:"sample_url"`
	PreviewURL string `json:"preview_url"`
	Tags       string `json:"tags"`
}

type tagJSON struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Type  int    `json:"type"`
}
