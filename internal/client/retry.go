package client

import (
	"net/http"
	"strconv"
	"time"
)

// maxAttempts is the total number of attempts (including the first one).
const maxAttempts = 3

// shouldRetry reports whether an HTTP status code warrants a retry.
// LinkedIn rate-limits with 429 and advertises transient 5xx as retryable.
func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

// retryDelay returns how long to wait before the next attempt.
// It honors the Retry-After header when present and otherwise falls back to
// exponential backoff starting at 200ms (200, 400, 800, ...).
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if s, err := strconv.Atoi(ra); err == nil {
				return time.Duration(s) * time.Second
			}
		}
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
