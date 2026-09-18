package gelbooru

import (
	"bytes"
	"context"
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
	Name           string
	BaseURL        string
	APIKey         string
	UserID         string
	MaxLimit       int
	CredentialEnvs []string
	HTTP           *fetch.Client
}

type Client struct {
	name       string
	baseURL    string
	apiKey     string
	userID     string
	maxLimit   int
	credential []string
	http       *fetch.Client
}

func New(cfg Config) *Client {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "gelbooru"
	}

	maxLimit := cfg.MaxLimit
	if maxLimit < 1 {
		maxLimit = 100
	}

	return &Client{
		name:       name,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		userID:     cfg.UserID,
		maxLimit:   maxLimit,
		credential: cfg.CredentialEnvs,
		http:       cfg.HTTP,
	}
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Capabilities() booru.Capabilities {
	return booru.Capabilities{Rating: true}
}

func (c *Client) Search(ctx context.Context, params booru.SearchParams) ([]booru.Post, error) {
	values := c.base("post")
	values.Set("tags", c.searchTerms(params))
	values.Set("limit", strconv.Itoa(c.limit(params.Limit)))

	if params.Page > 1 {
		values.Set("pid", strconv.Itoa(params.Page-1))
	}

	raw, err := getJSON[postResponse](ctx, c, values)
	if err != nil {
		return nil, err
	}

	posts := make([]booru.Post, 0, len(raw.Post))
	for _, item := range raw.Post {
		posts = append(posts, c.toPost(item))
	}

	return posts, nil
}

func (c *Client) Post(ctx context.Context, id string) (booru.Post, error) {
	values := c.base("post")
	values.Set("id", id)

	raw, err := getJSON[postResponse](ctx, c, values)
	if err != nil {
		return booru.Post{}, err
	}

	if len(raw.Post) == 0 {
		return booru.Post{}, fmt.Errorf("%s: post %s not found", c.name, id)
	}

	return c.toPost(raw.Post[0]), nil
}

func (c *Client) SearchTags(ctx context.Context, query booru.TagQuery) ([]booru.Tag, error) {
	values := c.base("tag")
	values.Set("orderby", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	if query.Query != "" {
		values.Set("name_pattern", "%"+query.Query+"%")
	}

	raw, err := getJSON[tagResponse](ctx, c, values)
	if err != nil {
		return nil, err
	}

	return c.toTags(raw.Tag), nil
}

func (c *Client) PopularTags(ctx context.Context, query booru.PopularQuery) ([]booru.Tag, error) {
	values := c.base("tag")
	values.Set("orderby", "count")
	values.Set("limit", strconv.Itoa(c.limit(query.Limit)))

	raw, err := getJSON[tagResponse](ctx, c, values)
	if err != nil {
		return nil, err
	}

	return c.toPopularTags(raw.Tag), nil
}

func (c *Client) base(section string) url.Values {
	return url.Values{
		"page": {"dapi"},
		"s":    {section},
		"q":    {"index"},
		"json": {"1"},
	}
}

func (c *Client) auth(values url.Values) {
	if c.apiKey != "" && c.userID != "" {
		values.Set("api_key", c.apiKey)
		values.Set("user_id", c.userID)
	}
}

func (c *Client) searchTerms(params booru.SearchParams) string {
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
		URL:        fmt.Sprintf("%s/index.php?page=post&s=view&id=%s", c.baseURL, id),
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

func (c *Client) toTags(raw []tagJSON) []booru.Tag {
	tags := make([]booru.Tag, 0, len(raw))

	for _, item := range raw {
		name := booru.NormalizeTag(item.Name)
		if name == "" {
			continue
		}

		tags = append(tags, booru.Tag{Name: name, Category: tagCategory(item.code()), Count: item.Count})
	}

	return tags
}

// Hashbooru's count ordering is dominated by deprecated alias stubs with inflated counts, so the popular snapshot drops
// them; a name search may still surface one as a normal tag result.
func (c *Client) toPopularTags(raw []tagJSON) []booru.Tag {
	tags := make([]booru.Tag, 0, len(raw))

	for _, item := range raw {
		name := booru.NormalizeTag(item.Name)
		if name == "" || item.deprecated() {
			continue
		}

		tags = append(tags, booru.Tag{Name: name, Category: tagCategory(item.code()), Count: item.Count})
	}

	return tags
}

func getJSON[T any](ctx context.Context, c *Client, values url.Values) (T, error) {
	c.auth(values)

	raw, err := fetch.GetJSON[T](ctx, c.http, c.name, "/index.php", values)
	if err != nil {
		return raw, c.translateError(err)
	}

	return raw, nil
}

func (c *Client) translateError(err error) error {
	var httpErr *fetch.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("%s: authentication failed; check %s", c.name, c.credentialList())
		}

		return err
	}

	var bodyErr *fetch.BodyError
	if errors.As(err, &bodyErr) && strings.Contains(strings.ToLower(bodyErr.Body), "authentication") {
		return fmt.Errorf("%s: authentication required; set %s", c.name, c.credentialList())
	}

	return err
}

func (c *Client) credentialList() string {
	if len(c.credential) == 0 {
		return "the client's credentials"
	}

	return strings.Join(c.credential, ", ")
}

// Gelbooru's post payload carries one space-separated tag string with no categories, so post tags are recorded as
// general. Category-accurate results only come from the tag search and popular endpoints.
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

const deprecatedTagType = 6

type postResponse struct {
	Post []postJSON `json:"post"`
}

// Most Hashbooru sites use the envelope, but Safebooru returns a bare array for the same request.
func (r *postResponse) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)

	if len(trimmed) > 0 && trimmed[0] == '[' {
		var posts []postJSON
		if err := json.Unmarshal(trimmed, &posts); err != nil {
			return err
		}

		r.Post = posts

		return nil
	}

	type envelope postResponse

	return json.Unmarshal(data, (*envelope)(r))
}

type tagResponse struct {
	Tag []tagJSON `json:"tag"`
}

type postJSON struct {
	ID         int    `json:"id"`
	CreatedAt  string `json:"created_at"`
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
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Type     int    `json:"type"`
	Category *int   `json:"category"`
}

func (t tagJSON) code() int {
	if t.Category != nil {
		return *t.Category
	}

	return t.Type
}

func (t tagJSON) deprecated() bool {
	return t.code() == deprecatedTagType
}
