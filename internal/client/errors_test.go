package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAPIError401(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"status":401,"code":"UNAUTHORIZED","message":"bad token","serviceErrorCode":65601}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/foo", nil, &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 401 {
		t.Errorf("status: %d", apiErr.Status)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("code: %q", apiErr.Code)
	}
	if apiErr.Message != "bad token" {
		t.Errorf("message: %q", apiErr.Message)
	}
	if apiErr.ServiceErrorCode != 65601 {
		t.Errorf("serviceErrorCode: %d", apiErr.ServiceErrorCode)
	}
	if !strings.Contains(apiErr.Error(), "auth login") {
		t.Errorf("401 error should hint at re-login, got: %v", apiErr)
	}
}

func TestAPIError403_HintsScope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"code":"ACCESS_DENIED","message":"insufficient scope"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/foo", nil, &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if !strings.Contains(apiErr.Error(), "scope") {
		t.Errorf("403 error should hint at missing scope, got: %v", apiErr)
	}
}

func TestAPIError_429WithRetryAfter(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		// Earlier attempts: Retry-After: 0 keeps the test fast.
		// Final attempt: advertise Retry-After: 5 so the surfaced error reflects it.
		if int(n) < 3 { // maxAttempts == 3
			w.Header().Set("Retry-After", "0")
		} else {
			w.Header().Set("Retry-After", "5")
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":429,"code":"RATE_LIMITED","message":"too fast"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/foo", nil, &out)
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 429 {
		t.Errorf("status: %d", apiErr.Status)
	}
	if apiErr.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter: got %v, want 5s", apiErr.RetryAfter)
	}
	if !strings.Contains(apiErr.Error(), "Retry-After: 5s") {
		t.Errorf("Error() should mention Retry-After: 5s, got: %v", apiErr)
	}
}

func TestAPIError_Unstructured(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`internal server error (plain text)`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/foo", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("should NOT be APIError (plain text response), got: %+v", apiErr)
	}
}
