package danbooru

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

const tagLimitError = "PostQuery::TagLimitError"

// Danbooru reports a rejected request as a JSON body whose message is the useful part and whose backtrace is not, so
// only the message is surfaced.
type apiErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (c *Client) apiError(err error) error {
	body, ok := parseAPIErrorBody(err)
	if !ok {
		return err
	}

	return fmt.Errorf("%s: %s", c.name, body.Message)
}

// searchError adds the terms actually sent, because Danbooru counts negated tags and order:random toward its limit
// while rating does not.
func (c *Client) searchError(err error, terms string) error {
	body, ok := parseAPIErrorBody(err)
	if !ok {
		return err
	}

	return fmt.Errorf("%s: %s (terms sent: %q%s)", c.name, body.Message, terms, c.tagLimitHint(body))
}

// validateTagLimit rejects an over-cap search before it reaches Danbooru. Danbooru counts every term except rating:
// metatags, which is why the count is taken over the terms as they will be sent.
func (c *Client) validateTagLimit(terms string) error {
	if c.tagLimit <= 0 {
		return nil
	}

	counted := countedTerms(terms)
	if counted <= c.tagLimit {
		return nil
	}

	return fmt.Errorf("%s: %d search terms exceed the %s tag limit of %d (rating: terms are not counted); %s (terms sent: %q)",
		c.name, counted, c.tierLabel(), c.tagLimit, c.tagLimitAdvice(), terms)
}

func countedTerms(terms string) int {
	count := 0

	for _, term := range strings.Fields(terms) {
		if strings.HasPrefix(term, "rating:") {
			continue
		}

		count++
	}

	return count
}

func (c *Client) tierLabel() string {
	if c.tier == "" {
		return "account"
	}

	return c.tier
}

func (c *Client) tagLimitAdvice() string {
	switch c.tier {
	case "anonymous", "member":
		return "set DANBOORU_LOGIN and DANBOORU_API_KEY to raise the limit to 6 with a Gold account"
	case "gold":
		return "set DANBOORU_TIER=platinum or builder for unlimited tags"
	default:
		return "remove search terms or raise the account tier"
	}
}

func (c *Client) tagLimitHint(body apiErrorBody) string {
	if body.Error != tagLimitError || c.authenticated() || len(c.credential) == 0 {
		return ""
	}

	return fmt.Sprintf("; %s raise the limit to 6 with a Gold account", strings.Join(c.credential, " and "))
}

func (c *Client) authenticated() bool {
	return c.login != "" && c.apiKey != ""
}

func parseAPIErrorBody(err error) (apiErrorBody, bool) {
	var httpErr *fetch.HTTPError
	if !errors.As(err, &httpErr) {
		return apiErrorBody{}, false
	}

	var body apiErrorBody
	if json.Unmarshal([]byte(httpErr.Body), &body) != nil || strings.TrimSpace(body.Message) == "" {
		return apiErrorBody{}, false
	}

	return body, true
}
