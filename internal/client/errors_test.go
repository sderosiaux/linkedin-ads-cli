package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
