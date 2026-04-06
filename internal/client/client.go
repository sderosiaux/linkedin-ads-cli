// Package client is the single HTTP gateway to the LinkedIn Marketing API.
// Every command in the CLI routes through Client so that auth headers,
// LinkedIn-Version, Rest.li protocol, retries and error decoding are applied
// consistently.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Options configures a new Client.
type Options struct {
	// BaseURL is the API root, typically https://api.linkedin.com/rest.
	BaseURL string
	// Token is the OAuth bearer token sent on every request.
	Token string
	// APIVersion is the LinkedIn-Version header value (yyyymm).
	APIVersion string
	// HTTP is an optional custom http.Client. If nil a 30s-timeout client is used.
	HTTP *http.Client
	// Verbose enables HTTP request tracing on Logger (defaults to stderr).
	Verbose bool
	// Logger is the sink for verbose request traces. nil falls back to stderr.
	// Authorization headers are NEVER written to this writer.
	Logger io.Writer
}

// Client is a thin LinkedIn REST client. Safe for concurrent use.
type Client struct {
	base    string
	token   string
	version string
	http    *http.Client
	verbose bool
	logger  io.Writer
}

// BaseURL returns the configured base URL (e.g. https://api.linkedin.com/rest).
// Used by pagination to extract the path+query from absolute paging.links hrefs.
func (c *Client) BaseURL() string { return c.base }

// New returns a Client configured from o.
func New(o Options) *Client {
	h := o.HTTP
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	logger := o.Logger
	if logger == nil {
		logger = os.Stderr
	}
	return &Client{
		base:    o.BaseURL,
		token:   o.Token,
		version: o.APIVersion,
		http:    h,
		verbose: o.Verbose,
		logger:  logger,
	}
}

// formatBytes renders a byte count as a compact human-readable string.
// Negative values (Content-Length unknown) collapse to "-".
func formatBytes(n int64) string {
	if n < 0 {
		return "-"
	}
	const (
		kb = 1024
		mb = 1024 * 1024
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1fMB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1fKB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// logTrace writes one METHOD url (status, duration, bytes) line to the
// configured logger. Authorization headers are deliberately never sourced or
// written here — only method, url, status, duration, and content length.
func (c *Client) logTrace(method, urlStr string, status int, dur time.Duration, contentLength int64, err error) {
	if !c.verbose || c.logger == nil {
		return
	}
	ms := dur.Milliseconds()
	if err != nil {
		_, _ = fmt.Fprintf(c.logger, "%s %s (err: %v, %dms)\n", method, urlStr, err, ms)
		return
	}
	_, _ = fmt.Fprintf(c.logger, "%s %s (%d, %dms, %s)\n", method, urlStr, status, ms, formatBytes(contentLength))
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	return c.doWithHeaders(ctx, method, path, query, body, nil)
}

// doWithHeaders is the shared transport loop. extraHeaders are merged on top of
// the standard auth/version headers and override Content-Type if present.
func (c *Client) doWithHeaders(ctx context.Context, method, path string, query url.Values, body any, extraHeaders map[string]string) (*http.Response, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		req, rerr := http.NewRequestWithContext(ctx, method, u.String(), reader) //nolint:gosec // base URL from trusted config
		if rerr != nil {
			return nil, rerr
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Linkedin-Version", c.version)
		req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}

		started := time.Now()
		resp, err = c.http.Do(req)
		if err != nil {
			c.logTrace(method, u.String(), 0, time.Since(started), -1, err)
			return nil, err
		}
		c.logTrace(method, u.String(), resp.StatusCode, time.Since(started), resp.ContentLength, nil)
		if !shouldRetry(resp.StatusCode) || attempt == maxAttempts-1 {
			return resp, nil
		}
		d := retryDelay(resp, attempt)
		_ = resp.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
	return resp, nil
}

// GetJSON issues a GET and decodes the JSON response body into out.
func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return parseError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// PostJSON sends a POST with a JSON body. If out is non-nil and the response
// has a body, the body is decoded into it. Returns the X-LinkedIn-Id header
// value (the new resource id) when present.
func (c *Client) PostJSON(ctx context.Context, path string, body, out any) (string, error) {
	resp, err := c.do(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return "", parseError(resp)
	}
	newID := resp.Header.Get("X-RestLi-Id")
	if newID == "" {
		newID = resp.Header.Get("X-LinkedIn-Id")
	}
	if out != nil && resp.ContentLength != 0 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return newID, err
		}
	}
	return newID, nil
}

// PartialUpdate sends a Rest.li 2.0 partial update: an HTTP POST carrying the
// X-RestLi-Method: PARTIAL_UPDATE header. Body should be {"patch":{"$set":...}}.
func (c *Client) PartialUpdate(ctx context.Context, path string, body any) error {
	resp, err := c.doWithHeaders(ctx, http.MethodPost, path, nil, body, map[string]string{
		"X-RestLi-Method": "PARTIAL_UPDATE",
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return parseError(resp)
	}
	return nil
}

// Delete sends a DELETE request. Returns nil on any 2xx response.
func (c *Client) Delete(ctx context.Context, path string) error {
	resp, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return parseError(resp)
	}
	return nil
}

// GetJSONRawQuery is like GetJSON but takes an already-encoded query string
// and forwards it verbatim. Use this for LinkedIn Rest.li finder parameters
// whose tuple syntax (e.g. "(start:(year:2026,...))" or "List(urn:...)")
// must NOT be percent-escaped — Go's url.Values.Encode() would mangle them.
func (c *Client) GetJSONRawQuery(ctx context.Context, path, rawQuery string, out any) error {
	resp, err := c.doRaw(ctx, http.MethodGet, path, rawQuery)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return parseError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// doRaw is a sibling of do() that sets u.RawQuery directly from rawQuery
// instead of going through url.Values.Encode(). It shares the same retry,
// header, and context handling.
func (c *Client) doRaw(ctx context.Context, method, path, rawQuery string) (*http.Response, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = rawQuery

	var resp *http.Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, rerr := http.NewRequestWithContext(ctx, method, u.String(), nil) //nolint:gosec // base URL from trusted config
		if rerr != nil {
			return nil, rerr
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Linkedin-Version", c.version)
		req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
		req.Header.Set("Accept", "application/json")

		started := time.Now()
		resp, err = c.http.Do(req)
		if err != nil {
			c.logTrace(method, u.String(), 0, time.Since(started), -1, err)
			return nil, err
		}
		c.logTrace(method, u.String(), resp.StatusCode, time.Since(started), resp.ContentLength, nil)
		if !shouldRetry(resp.StatusCode) || attempt == maxAttempts-1 {
			return resp, nil
		}
		d := retryDelay(resp, attempt)
		_ = resp.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
	return resp, nil
}
