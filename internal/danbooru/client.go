package danbooru

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
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
		maxLimit = 100
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

func (c *Client) Capabilities() booru.Capabilities {
	return booru.Capabilities{Rating: true, Random: true}
}

func (c *Client) Search(ctx context.Context, params booru.SearchParams) ([]booru.Post, error) {
	query := url.Values{}
	query.Set("tags", c.searchTerms(params))
	query.Set("limit", strconv.Itoa(c.limit(params.Limit)))

	if params.Page > 1 {
		query.Set("page", strconv.Itoa(params.Page))
	}

	c.auth(query)

	raw, err := fetch.GetJSON[[]postJSON](ctx, c.http, c.name, "/posts.json", query)
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
	raw, err := fetch.GetJSON[postJSON](ctx, c.http, c.name, "/posts/"+url.PathEscape(id)+".json", nil)
	if err != nil {
		return booru.Post{}, err
	}

	return c.toPost(raw), nil
}

func (c *Client) SearchTags(ctx context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	params := url.Values{}
	params.Set("search[order]", "count")
	params.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	if query.Query != "" {
		params.Set("search[name_matches]", wildcard(query.Query))
	}

	if query.Category != "" {
		params.Set("search[category]", strconv.Itoa(categoryCode(query.Category)))
	}

	c.auth(params)

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tags.json", params)
	if err != nil {
		return nil, err
	}

	tags := make([]booru.Tag, 0, len(raw))
	for _, item := range raw {
		tags = append(tags, booru.Tag{
			Name:     booru.NormalizeTag(item.Name),
			Category: tagCategory(item.Category),
			Count:    item.PostCount,
		})
	}

	return tags, nil
}

func (c *Client) PopularTags(ctx context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	params := url.Values{}
	params.Set("search[order]", "count")
	params.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	if query.Category != "" {
		params.Set("search[category]", strconv.Itoa(categoryCode(query.Category)))
	}

	c.auth(params)

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tags.json", params)
	if err != nil {
		return nil, err
	}

	tags := make([]booru.Tag, 0, len(raw))
	for _, item := range raw {
		tags = append(tags, booru.Tag{
			Name:     booru.NormalizeTag(item.Name),
			Category: tagCategory(item.Category),
			Count:    item.PostCount,
		})
	}

	return tags, nil
}

func (c *Client) RelatedTags(ctx context.Context, query booru.RelatedQuery) ([]booru.RelatedTag, error) {
	params := url.Values{"query": {query.Tag}}
	if query.Limit > 0 {
		params.Set("limit", strconv.Itoa(c.limit(query.Limit)))
	}

	c.auth(params)

	raw, err := fetch.GetJSON[relatedResponse](ctx, c.http, c.name, "/related_tag.json", params)
	if err != nil {
		return nil, err
	}

	tags := make([]booru.RelatedTag, 0, len(raw.RelatedTags))

	for i, item := range raw.RelatedTags {
		tags = append(tags, booru.RelatedTag{
			Tag:    booru.NormalizeTag(item.Tag),
			Client: c.name,
			Score:  item.Frequency,
			Rank:   i + 1,
		})
	}

	return tags, nil
}

func (c *Client) searchTerms(params booru.SearchParams) string {
	parts := make([]string, 0, len(params.Exclude)+2)

	if trimmed := strings.TrimSpace(params.Tags); trimmed != "" {
		parts = append(parts, trimmed)
	}

	if rating := ratingCode(params.Rating); rating != "" {
		parts = append(parts, "rating:"+rating)
	}

	for _, exclude := range params.Exclude {
		if tag := booru.NormalizeTag(exclude); tag != "" {
			parts = append(parts, "-"+tag)
		}
	}

	if params.Random {
		parts = append(parts, "order:random")
	}

	return strings.Join(parts, " ")
}

func (c *Client) auth(params url.Values) {
	if c.login != "" && c.apiKey != "" {
		params.Set("login", c.login)
		params.Set("api_key", c.apiKey)
	}
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
		URL:        fmt.Sprintf("%s/posts/%s", c.baseURL, id),
		FileURL:    raw.FileURL,
		SampleURL:  raw.LargeFileURL,
		PreviewURL: raw.PreviewFileURL,
		Width:      raw.ImageWidth,
		Height:     raw.ImageHeight,
		Rating:     parseRating(raw.Rating),
		Score:      raw.Score,
		FavCount:   raw.FavCount,
		Source:     raw.Source,
		Tags:       postTags(raw),
		CreatedAt:  parseTime(raw.CreatedAt),
	}
}

func postTags(raw postJSON) []booru.Tag {
	grouped := []struct {
		category booru.TagCategory
		value    string
	}{
		{booru.CategoryGeneral, raw.TagStringGeneral},
		{booru.CategoryArtist, raw.TagStringArtist},
		{booru.CategoryCopyright, raw.TagStringCopyright},
		{booru.CategoryCharacter, raw.TagStringCharacter},
		{booru.CategoryMeta, raw.TagStringMeta},
	}

	var tags []booru.Tag

	for _, entry := range grouped {
		for _, name := range strings.Fields(entry.value) {
			if normalized := booru.NormalizeTag(name); normalized != "" {
				tags = append(tags, booru.Tag{Name: normalized, Category: entry.category})
			}
		}
	}

	return tags
}

func wildcard(value string) string {
	value = strings.TrimSpace(value)

	if strings.ContainsAny(value, "*?") {
		return value
	}

	return "*" + value + "*"
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}

	return parsed.UTC()
}

type postJSON struct {
	ID                 int    `json:"id"`
	CreatedAt          string `json:"created_at"`
	Rating             string `json:"rating"`
	Score              int    `json:"score"`
	FavCount           int    `json:"fav_count"`
	Source             string `json:"source"`
	FileURL            string `json:"file_url"`
	LargeFileURL       string `json:"large_file_url"`
	PreviewFileURL     string `json:"preview_file_url"`
	ImageWidth         int    `json:"image_width"`
	ImageHeight        int    `json:"image_height"`
	TagStringGeneral   string `json:"tag_string_general"`
	TagStringArtist    string `json:"tag_string_artist"`
	TagStringCopyright string `json:"tag_string_copyright"`
	TagStringCharacter string `json:"tag_string_character"`
	TagStringMeta      string `json:"tag_string_meta"`
}

type tagJSON struct {
	Name      string `json:"name"`
	Category  int    `json:"category"`
	PostCount int    `json:"post_count"`
}

type relatedJSON struct {
	Tag          string `json:"tag"`
	Frequency    int    `json:"frequency"`
	CoOccurrence int    `json:"co_occurrence"`
}

type relatedResponse struct {
	RelatedTags []relatedJSON `json:"related_tags"`
}
