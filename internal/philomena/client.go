package philomena

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
	APIKey   string
	MaxLimit int
	HTTP     *fetch.Client
}

type Client struct {
	name     string
	baseURL  string
	apiKey   string
	maxLimit int
	http     *fetch.Client
}

func New(cfg Config) *Client {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "derpibooru"
	}

	maxLimit := cfg.MaxLimit
	if maxLimit < 1 {
		maxLimit = 100
	}

	return &Client{
		name:     name,
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		maxLimit: maxLimit,
		http:     cfg.HTTP,
	}
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Capabilities() booru.Capabilities {
	return booru.Capabilities{}
}

func (c *Client) Search(ctx context.Context, params booru.SearchParams) ([]booru.Post, error) {
	values := url.Values{}
	values.Set("q", c.filter(params))
	values.Set("per_page", strconv.Itoa(c.limit(params.Limit)))

	if params.Page > 1 {
		values.Set("page", strconv.Itoa(params.Page))
	}

	raw, err := getJSON[postsResponse](ctx, c, "/api/v1/json/search/posts", values)
	if err != nil {
		return nil, err
	}

	posts := make([]booru.Post, 0, len(raw.Posts))
	for _, item := range raw.Posts {
		posts = append(posts, c.toPost(item))
	}

	return posts, nil
}

func (c *Client) Post(ctx context.Context, id string) (booru.Post, error) {
	values := url.Values{"q": {"id:" + id}}

	raw, err := getJSON[postsResponse](ctx, c, "/api/v1/json/search/posts", values)
	if err != nil {
		return booru.Post{}, err
	}

	if len(raw.Posts) == 0 {
		return booru.Post{}, fmt.Errorf("%s: post %s not found", c.name, id)
	}

	return c.toPost(raw.Posts[0]), nil
}

func (c *Client) SearchTags(ctx context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("per_page", strconv.Itoa(c.limit(query.Limit)))

	if query.Query != "" {
		values.Set("q", wildcard(query.Query))
	}

	raw, err := getJSON[tagsResponse](ctx, c, "/api/v1/json/search/tags", values)
	if err != nil {
		return nil, err
	}

	return toTags(raw.Tags), nil
}

func (c *Client) PopularTags(ctx context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("per_page", strconv.Itoa(c.limit(query.Limit)))

	raw, err := getJSON[tagsResponse](ctx, c, "/api/v1/json/search/tags", values)
	if err != nil {
		return nil, err
	}

	return toTags(raw.Tags), nil
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

	width, height := raw.Width, raw.Height
	if width == 0 || height == 0 {
		if full, ok := raw.Representations["full"]; ok {
			width, height = full.Width, full.Height
		}
	}

	return booru.Post{
		Client:     c.name,
		ID:         id,
		URL:        fmt.Sprintf("%s/images/%s", c.baseURL, id),
		FileURL:    raw.representation("full"),
		SampleURL:  raw.representation("large"),
		PreviewURL: raw.representation("medium", "small", "thumb"),
		Width:      width,
		Height:     height,
		Rating:     booru.RatingExplicit,
		Score:      raw.Score,
		Source:     raw.source(),
		Tags:       postTags(raw.Tags),
		CreatedAt:  parseTime(raw.CreatedAt),
	}
}

func (c *Client) filter(params booru.SearchParams) string {
	parts := make([]string, 0, len(params.Exclude)+1)

	if trimmed := strings.TrimSpace(params.Tags); trimmed != "" {
		parts = append(parts, strings.Join(strings.Fields(trimmed), ","))
	}

	for _, exclude := range params.Exclude {
		if tag := booru.NormalizeTag(exclude); tag != "" {
			parts = append(parts, "-"+tag)
		}
	}

	return strings.Join(parts, ",")
}

func getJSON[T any](ctx context.Context, c *Client, path string, values url.Values) (T, error) {
	if c.apiKey != "" {
		values.Set("key", c.apiKey)
	}

	return fetch.GetJSON[T](ctx, c.http, c.name, path, values)
}

type postsResponse struct {
	Posts []postJSON `json:"posts"`
}

type tagsResponse struct {
	Tags []tagJSON `json:"tags"`
}

type postJSON struct {
	ID              int                       `json:"id"`
	CreatedAt       string                    `json:"created_at"`
	Width           int                       `json:"width"`
	Height          int                       `json:"height"`
	Score           int                       `json:"score"`
	Source          string                    `json:"source"`
	Sources         []string                  `json:"sources"`
	Tags            []string                  `json:"tags"`
	Representations map[string]representation `json:"representations"`
}

type representation struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (p postJSON) representation(keys ...string) string {
	for _, key := range keys {
		if rep, ok := p.Representations[key]; ok && rep.URL != "" {
			return rep.URL
		}
	}

	return ""
}

func (p postJSON) source() string {
	if trimmed := strings.TrimSpace(p.Source); trimmed != "" {
		return trimmed
	}

	return strings.Join(nonEmpty(p.Sources), "; ")
}

type tagJSON struct {
	Name   string `json:"name"`
	Images int    `json:"images"`
	Count  int    `json:"count"`
}

func (t tagJSON) total() int {
	if t.Images > 0 {
		return t.Images
	}

	return t.Count
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))

	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func wildcard(value string) string {
	value = strings.TrimSpace(value)

	if strings.ContainsAny(value, "*?") {
		return value
	}

	return "*" + value + "*"
}
