package e621

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
		name = "e621"
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
	return booru.Capabilities{Rating: true}
}

func (c *Client) Search(ctx context.Context, params booru.SearchParams) ([]booru.Post, error) {
	values := url.Values{}
	values.Set("tags", searchTerms(params))
	values.Set("limit", strconv.Itoa(c.limit(params.Limit)))

	if params.Page > 1 {
		values.Set("page", strconv.Itoa(params.Page))
	}

	raw, err := getJSON[postsResponse](ctx, c, "/posts.json", values)
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
	raw, err := getJSON[postResponse](ctx, c, "/posts/"+url.PathEscape(id)+".json", nil)
	if err != nil {
		return booru.Post{}, err
	}

	return c.toPost(raw.Post), nil
}

func (c *Client) SearchTags(ctx context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("search[order]", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	if query.Query != "" {
		values.Set("search[name_matches]", wildcard(query.Query))
	}

	raw, err := getJSON[tagsResponse](ctx, c, "/tags.json", values)
	if err != nil {
		return nil, err
	}

	tags := make([]booru.Tag, 0, len(raw.Tags))
	for _, item := range raw.Tags {
		tags = append(tags, booru.Tag{
			Name:     booru.NormalizeTag(item.Name),
			Category: tagCategory(item.Category),
			Count:    item.PostCount,
		})
	}

	return tags, nil
}

func (c *Client) PopularTags(ctx context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	values := url.Values{}
	values.Set("search[order]", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	raw, err := getJSON[tagsResponse](ctx, c, "/tags.json", values)
	if err != nil {
		return nil, err
	}

	tags := make([]booru.Tag, 0, len(raw.Tags))
	for _, item := range raw.Tags {
		tags = append(tags, booru.Tag{
			Name:     booru.NormalizeTag(item.Name),
			Category: tagCategory(item.Category),
			Count:    item.PostCount,
		})
	}

	return tags, nil
}

func (c *Client) RelatedTags(ctx context.Context, query booru.RelatedQuery) ([]booru.RelatedTag, error) {
	values := url.Values{"query": {query.Tag}}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(c.limit(query.Limit)))
	}

	raw, err := getJSON[json.RawMessage](ctx, c, "/related_tag.json", values)
	if err != nil {
		return nil, err
	}

	items, err := decodeRelated(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: decode /related_tag.json response: %w", c.name, err)
	}

	tags := make([]booru.RelatedTag, 0, len(items))

	for i, item := range items {
		tags = append(tags, booru.RelatedTag{
			Tag:    booru.NormalizeTag(item.Tag),
			Client: c.name,
			Score:  item.Frequency,
			Rank:   i + 1,
		})
	}

	return tags, nil
}

func (c *Client) toPost(raw postJSON) booru.Post {
	id := strconv.Itoa(raw.ID)

	return booru.Post{
		Client:     c.name,
		ID:         id,
		URL:        fmt.Sprintf("%s/posts/%s", c.baseURL, id),
		FileURL:    raw.File.URL,
		SampleURL:  raw.Sample.URL,
		PreviewURL: raw.Preview.URL,
		Width:      raw.File.Width,
		Height:     raw.File.Height,
		Rating:     parseRating(raw.Rating),
		Score:      raw.Score.Total,
		FavCount:   raw.FavCount,
		Source:     strings.Join(nonEmpty(raw.Sources), "; "),
		Tags:       postTags(raw.Tags),
		CreatedAt:  parseTime(raw.CreatedAt),
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

func (c *Client) headers() http.Header {
	if c.login == "" || c.apiKey == "" {
		return nil
	}

	token := base64.StdEncoding.EncodeToString([]byte(c.login + ":" + c.apiKey))

	return http.Header{"Authorization": {"Basic " + token}}
}

func getJSON[T any](ctx context.Context, c *Client, path string, values url.Values) (T, error) {
	raw, err := fetch.GetJSONWithHeaders[T](ctx, c.http, c.name, path, values, c.headers())
	if err != nil {
		var httpErr *fetch.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusForbidden {
			return raw, fmt.Errorf("%s: the request was rejected; a descriptive USER_AGENT with contact information is required (%w)", c.name, err)
		}

		return raw, err
	}

	return raw, nil
}

func decodeRelated(raw json.RawMessage) ([]relatedJSON, error) {
	var wrapper struct {
		RelatedTags []relatedJSON `json:"related_tags"`
	}

	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.RelatedTags != nil {
		return wrapper.RelatedTags, nil
	}

	var array []relatedJSON
	if err := json.Unmarshal(raw, &array); err != nil {
		return nil, err
	}

	return array, nil
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

type postsResponse struct {
	Posts []postJSON `json:"posts"`
}

type postResponse struct {
	Post postJSON `json:"post"`
}

type tagsResponse struct {
	Tags []tagJSON `json:"tags"`
}

type postJSON struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
	Rating    string `json:"rating"`
	Score     struct {
		Total int `json:"total"`
	} `json:"score"`
	FavCount int      `json:"fav_count"`
	Sources  []string `json:"sources"`
	File     struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"file"`
	Preview struct {
		URL string `json:"url"`
	} `json:"preview"`
	Sample struct {
		URL string `json:"url"`
	} `json:"sample"`
	Tags tagGroups `json:"tags"`
}

type tagGroups struct {
	General   []string `json:"general"`
	Artist    []string `json:"artist"`
	Character []string `json:"character"`
	Copyright []string `json:"copyright"`
	Species   []string `json:"species"`
	Meta      []string `json:"meta"`
	Lore      []string `json:"lore"`
	Invalid   []string `json:"invalid"`
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
