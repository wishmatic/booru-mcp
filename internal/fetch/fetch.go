package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wishmatic/booru-mcp/internal/utils"
)

const (
	maxAttempts = 3
	baseBackoff = 250 * time.Millisecond
	maxBackoff  = 2 * time.Second
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

func (e *HTTPError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("%s %s %s returned HTTP %d (%s)", e.Label, e.Method, e.Path, e.StatusCode, e.Status)
	}

	if e.Body != "" {
		return fmt.Sprintf("%s %s %s returned HTTP %d: %s", e.Label, e.Method, e.Path, e.StatusCode, e.Body)
	}

	return fmt.Sprintf("%s %s %s returned HTTP %d", e.Label, e.Method, e.Path, e.StatusCode)
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

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("%s: decode %s response: %w", label, path, err)
	}

	return out, nil
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
