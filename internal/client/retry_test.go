package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRetryOn429ThenSuccess(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"ok":"yes"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out map[string]string
	if err := c.GetJSON(context.Background(), "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
	if out["ok"] != "yes" {
		t.Errorf("body: %+v", out)
	}
}

func TestRetryGivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(503)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/x", nil, &out)
	if err == nil {
		t.Fatal("expected final error")
	}
	if calls.Load() != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", calls.Load())
	}
}

func TestNoRetryOn400(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = calls.Add(1)
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"status":400,"code":"BAD","message":"nope"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/x", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", calls.Load())
	}
}
