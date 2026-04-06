package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("linkedin api %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("linkedin api %d: %s", e.Status, e.Message)
}

// parseError reads resp.Body and returns an *APIError when the body matches
// LinkedIn's envelope, otherwise a plain error with the raw payload.
// The caller retains ownership of resp.Body and must close it.
func parseError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var api APIError
	if json.Unmarshal(b, &api) == nil && api.Status != 0 {
		return &api
	}
	return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
}
