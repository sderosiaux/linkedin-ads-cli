package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListLeadForms(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/leadGenForms" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "account" {
			t.Errorf("q: %q", q.Get("q"))
		}
		if q.Get("account") != "urn:li:sponsoredAccount:12345" {
			t.Errorf("account: %q", q.Get("account"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             1,
					"name":           "EBook download",
					"status":         "ACTIVE",
					"account":        "urn:li:sponsoredAccount:12345",
					"locale":         map[string]any{"country": "US", "language": "en"},
					"headline":       "Get the eBook",
					"description":    "Free download",
					"createdAt":      1700000000000,
					"lastModifiedAt": 1710000000000,
				},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	forms, err := ListLeadForms(context.Background(), c, "12345", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 1 {
		t.Fatalf("len: %d", len(forms))
	}
	f := forms[0]
	if f.ID != 1 || f.Name != "EBook download" {
		t.Errorf("form: %+v", f)
	}
	if f.Locale == nil || f.Locale.Country != "US" || f.Locale.Language != "en" {
		t.Errorf("locale: %+v", f.Locale)
	}
	if f.Headline != "Get the eBook" {
		t.Errorf("headline: %q", f.Headline)
	}
}
