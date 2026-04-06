// Package client is the single HTTP gateway to the LinkedIn Marketing API.
// Every command in the CLI routes through Client so that auth headers,
// LinkedIn-Version, Rest.li protocol, retries and error decoding are applied
// consistently.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
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
	// Verbose reserves a hook for request tracing (unused today).
	Verbose bool
}

// Client is a thin LinkedIn REST client. Safe for concurrent use.
type Client struct {
	base    string
	token   string
	version string
	http    *http.Client
	verbose bool
}

// New returns a Client configured from o.
func New(o Options) *Client {
	h := o.HTTP
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		base:    o.BaseURL,
		token:   o.Token,
		version: o.APIVersion,
		http:    h,
		verbose: o.Verbose,
	}
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader) //nolint:gosec // base URL from trusted config
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Linkedin-Version", c.version)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
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
