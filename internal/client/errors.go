package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// APIError is the parsed LinkedIn API error envelope. LinkedIn returns errors
// as JSON like:
//
//	{"status":401,"code":"UNAUTHORIZED","message":"...","serviceErrorCode":65601}
//
// Callers can use errors.As to detect and inspect these.
type APIError struct {
	Status           int    `json:"status"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	ServiceErrorCode int    `json:"serviceErrorCode"`
	// RetryAfter is the Retry-After header value, populated only on 429
	// responses returned to the caller after retry exhaustion.
	RetryAfter time.Duration `json:"-"`
}

func (e *APIError) Error() string {
	var base string
	if e.Code != "" {
		base = fmt.Sprintf("linkedin api %d %s: %s", e.Status, e.Code, e.Message)
	} else {
		base = fmt.Sprintf("linkedin api %d: %s", e.Status, e.Message)
	}
	switch e.Status {
	case http.StatusUnauthorized:
		return base + " — run 'linkedin-ads auth login' to refresh your token"
	case http.StatusForbidden:
		return base + " — token is missing the required scope for this endpoint"
	case http.StatusTooManyRequests:
		if e.RetryAfter > 0 {
			return fmt.Sprintf("%s — Retry-After: %s", base, e.RetryAfter)
		}
		return base
	}
	return base
}

// parseError reads resp.Body and returns an *APIError when the body matches
// LinkedIn's envelope, otherwise a plain error with the raw payload.
// On 429 responses, the Retry-After header is parsed into APIError.RetryAfter
// so callers can surface it to users after retry exhaustion.
// The caller retains ownership of resp.Body and must close it.
func parseError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var api APIError
	if json.Unmarshal(b, &api) == nil && api.Status != 0 {
		if api.Status == http.StatusTooManyRequests {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, err := strconv.Atoi(ra); err == nil {
					api.RetryAfter = time.Duration(n) * time.Second
				}
			}
		}
		return &api
	}
	return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
}
