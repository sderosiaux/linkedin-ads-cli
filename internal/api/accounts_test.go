package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListAccounts(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "search" {
			t.Errorf("expected q=search, got %q", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 12345, "name": "Acme EMEA", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
				{"id": 67890, "name": "Acme US", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
			},
			"paging": map[string]any{"start": 0, "count": 2, "total": 2},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	accts, err := ListAccounts(context.Background(), c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 2 {
		t.Fatalf("len: %d", len(accts))
	}
	if accts[0].ID != 12345 || accts[0].Name != "Acme EMEA" || accts[0].Currency != "USD" {
		t.Errorf("account[0]: %+v", accts[0])
	}
}

func TestListAccounts_Limit(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 1, "name": "A", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
				{"id": 2, "name": "B", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
				{"id": 3, "name": "C", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
			},
			"paging": map[string]any{"start": 0, "count": 3, "total": 3},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	accts, err := ListAccounts(context.Background(), c, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 2 {
		t.Errorf("expected 2 with limit, got %d", len(accts))
	}
}

func TestGetAccount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/12345" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":12345,"name":"Acme","status":"ACTIVE","type":"BUSINESS","currency":"USD"}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	acct, err := GetAccount(context.Background(), c, "12345")
	if err != nil {
		t.Fatal(err)
	}
	if acct.ID != 12345 {
		t.Errorf("id: %d", acct.ID)
	}
}
