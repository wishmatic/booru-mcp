package booru

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wishmatic/booru-mcp/internal/fetch"
)

// Danbooru reports a rejected request as a JSON body whose message is the useful part and whose backtrace is not, so
// only the message is surfaced.
type apiErrorBody struct {
	Message string `json:"message"`
}

func (c *Client) apiError(err error) error {
	body, ok := parseAPIErrorBody(err)
	if !ok {
		return err
	}

	return fmt.Errorf("%s: %s", c.name, body.Message)
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
