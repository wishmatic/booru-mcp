package booru

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

const (
	DefaultBaseURL = "https://danbooru.donmai.us"

	// tagPageCeiling is the largest page requested from the tag listing. Keeping it modest bounds how many pages one
	// window spans and keeps the page index small, which matters because Danbooru caps how deep page may go.
	tagPageCeiling = 100

	tagFields = "name,category,post_count"

	implicationPageCeiling = 1000
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
//
// Matches below query.MinCount are withheld rather than returned. Danbooru's tag table is user-editable and carries
// concatenated and punctuation-mangled zero-post rows that no caller wants, but those rows are real canonical tags, so
// the page reports how many were withheld instead of pretending they do not exist. Because the listing is
// count-ordered they are always the tail, so paging stops as soon as one is seen.
func (c *Client) SearchTags(ctx context.Context, query TagQuery) (TagPage, error) {
	if strings.TrimSpace(query.Search) == "" {
		return TagPage{}, fmt.Errorf("%s: tag search needs a search term", c.name)
	}

	if query.Limit < 1 {
		return TagPage{}, fmt.Errorf("%s: tag search needs a positive limit, got %d", c.name, query.Limit)
	}

	floor := query.MinCount
	if floor < 0 {
		floor = 0
	}

	pageSize := c.pageSize()
	skip := query.Offset % pageSize

	// One item past the window decides whether more matches exist, so the window is read wider than it is returned.
	wanted := skip + query.Limit + 1
	collected := make([]Tag, 0, wanted)

	window := TagPage{}

	for page := query.Offset/pageSize + 1; len(collected) < wanted; page++ {
		batch, err := c.searchPage(ctx, query, page, pageSize)
		if err != nil {
			return TagPage{}, err
		}

		exhausted := len(batch) < pageSize

		for _, tag := range batch {
			if tag.Count < floor {
				exhausted = true
				window.Withheld++

				if tag.Count > window.WithheldBest {
					window.WithheldBest = tag.Count
				}

				continue
			}

			collected = append(collected, tag)
		}

		if exhausted {
			break
		}
	}

	sortTags(collected)

	if skip < len(collected) {
		window.Tags = collected[skip:]
	}

	if len(window.Tags) > query.Limit {
		window.More = true
		window.Tags = window.Tags[:query.Limit]
	}

	return window, nil
}

func (c *Client) searchPage(ctx context.Context, query TagQuery, page, limit int) ([]Tag, error) {
	params := url.Values{}
	params.Set("search[order]", "count")
	params.Set("search[name_matches]", "*"+query.Search+"*")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("only", tagFields)
	c.setCategories(params, query.Categories)

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

func (c *Client) setCategories(params url.Values, categories []TagCategory) {
	if len(categories) == 0 {
		return
	}

	codes := make([]string, 0, len(categories))

	for _, category := range categories {
		codes = append(codes, strconv.Itoa(category.Code()))
	}

	params.Set("search[category]", strings.Join(codes, ","))
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

func sortTags(tags []Tag) {
	sort.SliceStable(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}

		return tags[i].Name < tags[j].Name
	})
}
