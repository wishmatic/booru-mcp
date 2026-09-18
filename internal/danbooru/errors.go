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
