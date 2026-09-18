package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wishmatic/booru-mcp/internal/utils"
)

const (
	maxAttempts    = 3
	baseBackoff    = 250 * time.Millisecond
	maxBackoff     = 2 * time.Second
	maxBodyExcerpt = 4 * 1024
)

type Limiter interface {
	Wait(ctx context.Context) error
}

type Config struct {
	BaseURL       string
	UserAgent     string
	Timeout       time.Duration
	Limiter       Limiter
	VerboseErrors bool
}

type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client
	limiter   Limiter
	verbose   bool
}

type HTTPError struct {
	Label      string
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string
}

// BodyError is a 200 response whose body is not the JSON document the caller asked for. It exists so a block page, a
// redirect to HTML, or an upstream message string is diagnosed instead of surfacing as a Go type error.
type BodyError struct {
	Label       string
	Path        string
	StatusCode  int
	ContentType string
	Body        string
	Reason      string
}

func (e *BodyError) Error() string {
	message := fmt.Sprintf("%s %s returned HTTP %d", e.Label, e.Path, e.StatusCode)

	if contentType := mediaType(e.ContentType); contentType != "" {
		message += " (" + contentType + ")"
	}

	message += " with " + e.Reason

	if e.Body != "" {
		message += ": " + e.Body
	}

	return message
}

func (e *HTTPError) Error() string {
	message := fmt.Sprintf("%s %s %s returned HTTP %d", e.Label, e.Method, e.Path, e.StatusCode)

	if e.Status != "" {
		message += fmt.Sprintf(" (%s)", e.Status)
	}

	if e.Body != "" {
		message += ": " + e.Body
	}

	return message
}

func New(cfg Config) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("fetch: invalid base URL %q", cfg.BaseURL)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("fetch: base URL scheme must be http or https, got %q", u.Scheme)
	}

	if cfg.Limiter == nil {
		return nil, errors.New("fetch: limiter is required")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	return &Client{
		baseURL:   strings.TrimRight(u.String(), "/"),
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: timeout},
		limiter:   cfg.Limiter,
		verbose:   cfg.VerboseErrors,
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func GetJSON[T any](ctx context.Context, c *Client, label, path string, query url.Values) (T, error) {
	return GetJSONWithHeaders[T](ctx, c, label, path, query, nil)
}

func GetJSONWithHeaders[T any](ctx context.Context, c *Client, label, path string, query url.Values, headers http.Header) (T, error) {
	var out T

	resp, err := c.get(ctx, label, path, query, headers)
	if err != nil {
		return out, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("%s: read %s response: %w", label, path, err)
	}

	trimmed := bytes.TrimSpace(body)

	if reason, ok := rejectBody(trimmed); ok {
		return out, c.bodyError(label, path, resp, excerpt(trimmed), reason)
	}

	if err := json.Unmarshal(trimmed, &out); err != nil {
		reason := fmt.Sprintf("a body that does not decode as the expected JSON shape (%v)", err)

		return out, c.bodyError(label, path, resp, excerpt(trimmed), reason)
	}

	return out, nil
}

func excerpt(body []byte) string {
	if len(body) > maxBodyExcerpt {
		return string(body[:maxBodyExcerpt])
	}

	return string(body)
}

// rejectBody reports whether a body cannot be a JSON document for these APIs, before any unmarshal is attempted.
func rejectBody(body []byte) (string, bool) {
	if len(body) == 0 {
		return "an empty body", true
	}

	switch body[0] {
	case '<':
		return "an HTML or XML body, not JSON", true

	case '"':
		var message string
		if json.Unmarshal(body, &message) == nil && message != "" {
			return "an upstream message instead of a JSON document", true
		}

		return "a JSON string body, not a JSON document", true

	case '{', '[':
		return "", false

	default:
		return "a non-JSON body", true
	}
}

func (c *Client) bodyError(label, path string, resp *http.Response, body, reason string) *BodyError {
	return &BodyError{
		Label:       label,
		Path:        path,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
		Reason:      reason,
	}
}

func mediaType(contentType string) string {
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		return strings.TrimSpace(contentType[:idx])
	}

	return strings.TrimSpace(contentType)
}

func (c *Client) get(ctx context.Context, label, path string, query url.Values, headers http.Header) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("%s: wait for rate limiter: %w", label, err)
		}

		resp, err := c.do(ctx, path, query, headers)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		body := utils.ReadLimited(resp.Body)
		_ = resp.Body.Close()

		lastErr = c.httpError(label, path, resp, body)

		if !retryable(resp.StatusCode) || attempt == maxAttempts-1 {
			break
		}

		if err := sleep(ctx, retryDelay(resp, attempt)); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *Client) do(ctx context.Context, path string, query url.Values, headers http.Header) (*http.Response, error) {
	rawURL := c.baseURL + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", path, err)
	}

	req.Header.Set("Accept", "application/json")

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return c.http.Do(req)
}

func (c *Client) httpError(label, path string, resp *http.Response, body string) *HTTPError {
	e := &HTTPError{
		Label:      label,
		Method:     http.MethodGet,
		Path:       path,
		StatusCode: resp.StatusCode,
		Body:       body,
	}

	if c.verbose {
		e.Status = resp.Status
	}

	return e
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if after := resp.Header.Get("Retry-After"); after != "" {
		if seconds, err := time.ParseDuration(after + "s"); err == nil {
			return seconds
		}

		if when, err := http.ParseTime(after); err == nil {
			if delay := time.Until(when); delay > 0 {
				return delay
			}

			return 0
		}
	}

	delay := baseBackoff << attempt
	if delay > maxBackoff {
		delay = maxBackoff
	}

	return delay
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}
