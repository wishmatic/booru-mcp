package booru

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

const relationFields = "antecedent_name,consequent_name"

// FindTag looks up one canonical tag by exact name. It reports false, with no error, when no such tag exists.
func (c *Client) FindTag(ctx context.Context, name string) (Tag, bool, error) {
	name = NormalizeTag(name)
	if name == "" {
		return Tag{}, false, fmt.Errorf("%s: tag lookup needs a name", c.name)
	}

	params := url.Values{}
	params.Set("search[name]", name)
	params.Set("only", tagFields)
	c.auth(params)

	raw, err := fetch.GetJSON[[]tagJSON](ctx, c.http, c.name, "/tags.json", params)
	if err != nil {
		return Tag{}, false, c.apiError(err)
	}

	tags := toTags(raw)
	if len(tags) == 0 {
		return Tag{}, false, nil
	}

	return tags[0], true, nil
}

// AliasTarget follows active aliases from name to the first name that is not itself aliased, reporting whether any alias
// was followed. It never loops, even on a cyclic alias chain.
func (c *Client) AliasTarget(ctx context.Context, name string) (string, bool, error) {
	name = NormalizeTag(name)
	if name == "" {
		return "", false, nil
	}

	aliased := false
	seen := map[string]bool{name: true}

	for {
		consequents, err := c.consequents(ctx, "/tag_aliases.json", name, 1)
		if err != nil {
			return "", false, err
		}

		if len(consequents) == 0 {
			break
		}

		aliased = true

		if seen[consequents[0]] {
			break
		}

		seen[consequents[0]] = true
		name = consequents[0]
	}

	if !aliased {
		return "", false, nil
	}

	return name, true, nil
}

// Implications returns the sorted canonical names implied by name.
func (c *Client) Implications(ctx context.Context, name string) ([]string, error) {
	name = NormalizeTag(name)
	if name == "" {
		return nil, nil
	}

	consequents, err := c.consequents(ctx, "/tag_implications.json", name, implicationPageCeiling)
	if err != nil {
		return nil, err
	}

	sort.Strings(consequents)

	return consequents, nil
}

// ImplicationPage reads one page of the active implication graph. The caller drives the page walk, so the crawl is
// owned by the index rather than by the client.
func (c *Client) ImplicationPage(ctx context.Context, page, limit int) ([]Implication, error) {
	params := url.Values{}
	params.Set("search[status]", "active")
	params.Set("only", relationFields)
	params.Set("limit", strconv.Itoa(limit))

	if page > 1 {
		params.Set("page", strconv.Itoa(page))
	}

	c.auth(params)

	raw, err := fetch.GetJSON[[]relationJSON](ctx, c.http, c.name, "/tag_implications.json", params)
	if err != nil {
		return nil, c.apiError(err)
	}

	relations := make([]Implication, 0, len(raw))

	for _, item := range raw {
		antecedent := NormalizeTag(item.AntecedentName)
		consequent := NormalizeTag(item.ConsequentName)

		if antecedent == "" || consequent == "" {
			continue
		}

		relations = append(relations, Implication{Antecedent: antecedent, Consequent: consequent})
	}

	return relations, nil
}

func (c *Client) consequents(ctx context.Context, path, antecedent string, limit int) ([]string, error) {
	params := url.Values{}
	params.Set("search[antecedent_name]", antecedent)
	params.Set("search[status]", "active")
	params.Set("only", relationFields)
	params.Set("limit", strconv.Itoa(limit))
	c.auth(params)

	raw, err := fetch.GetJSON[[]relationJSON](ctx, c.http, c.name, path, params)
	if err != nil {
		return nil, c.apiError(err)
	}

	out := make([]string, 0, len(raw))

	for _, item := range raw {
		if consequent := NormalizeTag(item.ConsequentName); consequent != "" {
			out = append(out, consequent)
		}
	}

	return out, nil
}
