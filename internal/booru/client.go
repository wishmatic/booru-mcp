package booru

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

const (
	DefaultBaseURL = "https://danbooru.donmai.us"

	// tagPageCeiling is the largest page requested from the tag listing. Keeping it modest bounds how many pages one
	// window spans and keeps the page index small, which matters because Danbooru caps how deep page may go.
	tagPageCeiling = 100
)

type Config struct {
	Name     string
	BaseURL  string
	Login    string
	APIKey   string
	MaxLimit int
	HTTP     *fetch.Client
}

type Client struct {
	name     string
	baseURL  string
	login    string
	apiKey   string
	maxLimit int
	http     *fetch.Client
}

func New(cfg Config) *Client {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "danbooru"
	}

	maxLimit := cfg.MaxLimit
	if maxLimit < 1 {
		maxLimit = tagPageCeiling
	}

	return &Client{
		name:     name,
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		login:    cfg.Login,
		apiKey:   cfg.APIKey,
		maxLimit: maxLimit,
		http:     cfg.HTTP,
	}
}

func (c *Client) Name() string {
	return c.name
}

// SearchTags returns exactly the window [query.Offset, query.Offset+query.Limit) of the count-ordered tag matches, or
// a shorter final window, plus whether at least one match exists past it. The listing is page-based, so the window is
// read from the one or two pages that contain it.
func (c *Client) SearchTags(ctx context.Context, query TagQuery) (TagPage, error) {
	if strings.TrimSpace(query.Search) == "" {
		return TagPage{}, fmt.Errorf("%s: tag search needs a search term", c.name)
	}

	if query.Limit < 1 {
		return TagPage{}, fmt.Errorf("%s: tag search needs a positive limit, got %d", c.name, query.Limit)
	}

	pageSize := c.pageSize()
	skip := query.Offset % pageSize

	// One item past the window decides whether more matches exist, so the window is read wider than it is returned.
	wanted := skip + query.Limit + 1
	collected := make([]Tag, 0, wanted)

	for page := query.Offset/pageSize + 1; len(collected) < wanted; page++ {
		batch, err := c.tagPage(ctx, query.Search, page, pageSize)
		if err != nil {
			return TagPage{}, err
		}

		collected = append(collected, batch...)

		if len(batch) < pageSize {
			break
		}
	}

	page := TagPage{}

	if skip < len(collected) {
		page.Tags = collected[skip:]
	}

	if len(page.Tags) > query.Limit {
		page.More = true
		page.Tags = page.Tags[:query.Limit]
	}

	return page, nil
}

func (c *Client) tagPage(ctx context.Context, search string, page, limit int) ([]Tag, error) {
	params := url.Values{}
	params.Set("search[order]", "count")
	params.Set("search[name_matches]", "*"+search+"*")
	params.Set("limit", strconv.Itoa(limit))

	if page > 1 {
		params.Set("page", strconv.Itoa(page))
	}

	c.auth(params)

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tags.json", params)
	if err != nil {
		return nil, c.apiError(err)
	}

	return toTags(raw), nil
}

func (c *Client) pageSize() int {
	if c.maxLimit > tagPageCeiling {
		return tagPageCeiling
	}

	return c.maxLimit
}

func (c *Client) auth(params url.Values) {
	if c.login == "" || c.apiKey == "" {
		return
	}

	params.Set("login", c.login)
	params.Set("api_key", c.apiKey)
}

func toTags(raw []tagJSON) []Tag {
	tags := make([]Tag, 0, len(raw))

	for _, item := range raw {
		name := NormalizeTag(item.Name)
		if name == "" {
			continue
		}

		tags = append(tags, Tag{Name: name, Category: tagCategory(item.Category), Count: item.PostCount})
	}

	return tags
}

type tagJSON struct {
	Name      string `json:"name"`
	Category  int    `json:"category"`
	PostCount int    `json:"post_count"`
}
