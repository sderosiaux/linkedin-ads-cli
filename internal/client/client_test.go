package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
