package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetSetsAuthAndVersionHeaders(t *testing.T) {
	t.Parallel()
	var gotAuth, gotVersion, gotRestli, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Linkedin-Version")
		gotRestli = r.Header.Get("X-Restli-Protocol-Version")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "yes"})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "tok", APIVersion: "202601"})
	var out map[string]string
	if err := c.GetJSON(context.Background(), "/ping", nil, &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth header: %q", gotAuth)
	}
	if gotVersion != "202601" {
		t.Errorf("version header: %q", gotVersion)
	}
	if gotRestli != "2.0.0" {
		t.Errorf("restli header: %q", gotRestli)
	}
	if gotAccept != "application/json" {
		t.Errorf("accept header: %q", gotAccept)
	}
	if out["ok"] != "yes" {
		t.Errorf("body: %+v", out)
	}
}

func TestGetJSONWithQueryParams(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	q := url.Values{}
	q.Set("q", "search")
	q.Set("start", "0")
	var out map[string]any
	if err := c.GetJSON(context.Background(), "/accounts", q, &out); err != nil {
		t.Fatal(err)
	}
	if gotQuery == "" || !strings.Contains(gotQuery, "q=search") {
		t.Errorf("query: %q", gotQuery)
	}
}

func TestPostJSONSendsBodyAndReturnsXLinkedInID(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotContentType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("X-LinkedIn-Id", "999")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	id, err := c.PostJSON(context.Background(), "/adCampaignGroups", map[string]any{
		"name":   "Q1",
		"status": "DRAFT",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups" {
		t.Errorf("path: %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type: %q", gotContentType)
	}
	var decoded map[string]any
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if decoded["name"] != "Q1" || decoded["status"] != "DRAFT" {
		t.Errorf("body: %+v", decoded)
	}
	if id != "999" {
		t.Errorf("id: %q", id)
	}
}

func TestPostJSONDecodesResponseWhenOutNonNil(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-LinkedIn-Id", "42")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42,"name":"hello"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	var out map[string]any
	id, err := c.PostJSON(context.Background(), "/adCampaignGroups", map[string]any{}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if id != "42" {
		t.Errorf("id: %q", id)
	}
	if out["name"] != "hello" {
		t.Errorf("decoded body: %+v", out)
	}
}

func TestPostJSONReturnsAPIErrorOn400(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"code":"BAD","message":"missing field"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if _, err := c.PostJSON(context.Background(), "/adCampaignGroups", map[string]any{}, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestPostJSONRetriesOn503(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("X-LinkedIn-Id", "5")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	id, err := c.PostJSON(context.Background(), "/adCampaignGroups", map[string]any{"name": "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
	if id != "5" {
		t.Errorf("id: %q", id)
	}
}

func TestPartialUpdateSendsRestliMethodHeader(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotRestliMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRestliMethod = r.Header.Get("X-RestLi-Method")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	body := map[string]any{
		"patch": map[string]any{
			"$set": map[string]any{"status": "ACTIVE"},
		},
	}
	if err := c.PartialUpdate(context.Background(), "/adCampaignGroups/123", body); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups/123" {
		t.Errorf("path: %q", gotPath)
	}
	if gotRestliMethod != "PARTIAL_UPDATE" {
		t.Errorf("X-RestLi-Method: %q", gotRestliMethod)
	}
	if !strings.Contains(string(gotBody), `"patch"`) || !strings.Contains(string(gotBody), `"$set"`) {
		t.Errorf("body: %s", string(gotBody))
	}
}

func TestPartialUpdateReturnsAPIErrorOn400(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"code":"BAD","message":"nope"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := c.PartialUpdate(context.Background(), "/adCampaignGroups/1", map[string]any{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteSendsDelete(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := c.Delete(context.Background(), "/adCampaignGroups/123"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups/123" {
		t.Errorf("path: %q", gotPath)
	}
}

func TestDeleteReturnsAPIErrorOn404(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"code":"NOT_FOUND","message":"missing"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := c.Delete(context.Background(), "/adCampaignGroups/x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetJSONRawQueryPreservesUnescapedTuples(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	raw := "q=analytics&pivot=CAMPAIGN&dateRange=(start:(year:2026,month:1,day:1),end:(year:2026,month:1,day:31))&accounts=List(urn:li:sponsoredAccount:12345)"
	var out map[string]any
	if err := c.GetJSONRawQuery(context.Background(), "/adAnalytics", raw, &out); err != nil {
		t.Fatal(err)
	}
	if gotQuery != raw {
		t.Errorf("expected raw query preserved.\n got: %s\nwant: %s", gotQuery, raw)
	}
}
