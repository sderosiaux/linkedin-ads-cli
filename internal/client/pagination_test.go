package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPaginateStartCount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		resp := map[string]any{
			"elements": []map[string]any{},
			"paging":   map[string]any{"start": 0, "count": 2, "total": 5},
		}
		switch start {
		case "", "0":
			resp["elements"] = []map[string]any{{"id": 1}, {"id": 2}}
		case "2":
			resp["elements"] = []map[string]any{{"id": 3}, {"id": 4}}
		case "4":
			resp["elements"] = []map[string]any{{"id": 5}}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateStartCount(context.Background(), c, "/items", nil, 2, 0, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 items, got %d: %v", len(all), all)
	}
	for i, item := range all {
		if item["id"].(float64) != float64(i+1) {
			t.Errorf("index %d: %v", i, item)
		}
	}
}

func TestPaginateStartCount_HonorsLimit(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{{"id": 1}, {"id": 2}},
			"paging":   map[string]any{"start": 0, "count": 2, "total": 100},
		})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateStartCount(context.Background(), c, "/items", nil, 2, 3, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items (limit), got %d", len(all))
	}
}

func TestPaginateStartCountRaw_PreservesTupleSyntax(t *testing.T) {
	t.Parallel()
	wantTuple := "search=(account:(values:List(urn:li:sponsoredAccount:12345)))"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, wantTuple) {
			t.Errorf("RawQuery missing unescaped tuple: got %q want substring %q", r.URL.RawQuery, wantTuple)
		}
		if !strings.Contains(r.URL.RawQuery, "start=0") || !strings.Contains(r.URL.RawQuery, "count=2") {
			t.Errorf("RawQuery missing pagination cursor: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{{"id": 1}},
			"paging":   map[string]any{"start": 0, "count": 2, "total": 1},
		})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	rawQuery := "q=search&search=(account:(values:List(urn:li:sponsoredAccount:12345)))"
	var all []map[string]any
	if err := PaginateStartCountRaw(context.Background(), c, "/items", rawQuery, 2, 0, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 item, got %d", len(all))
	}
}

func TestPaginateToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("pageToken")
		switch token {
		case "":
			_, _ = w.Write([]byte(`{"elements":[{"id":1}],"metadata":{"nextPageToken":"abc"}}`))
		case "abc":
			_, _ = w.Write([]byte(`{"elements":[{"id":2}],"metadata":{"nextPageToken":"def"}}`))
		case "def":
			_, _ = w.Write([]byte(`{"elements":[{"id":3}]}`))
		}
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateToken(context.Background(), c, "/x", nil, 0, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}

func TestPaginateToken_HonorsLimit(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[{"id":1},{"id":2}],"metadata":{"nextPageToken":"abc"}}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateToken(context.Background(), c, "/x", nil, 1, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
}
