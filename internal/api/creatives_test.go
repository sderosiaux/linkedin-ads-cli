package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListCreatives(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCreatives" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "q=criteria") {
			t.Errorf("RawQuery missing q=criteria: %q", raw)
		}
		wantTuple := "campaigns=List(urn:li:sponsoredCampaign:42)"
		if !strings.Contains(raw, wantTuple) {
			t.Errorf("RawQuery missing unescaped tuple %q: %q", wantTuple, raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             "urn:li:sponsoredCreative:1",
					"status":         "ACTIVE",
					"intendedStatus": "ACTIVE",
					"campaign":       "urn:li:sponsoredCampaign:42",
					"review":         "APPROVED",
					"createdAt":      1700000000000,
					"lastModifiedAt": 1710000000000,
				},
				{
					"id":             "urn:li:sponsoredCreative:2",
					"status":         "DRAFT",
					"intendedStatus": "DRAFT",
					"campaign":       "urn:li:sponsoredCampaign:42",
				},
			},
			"paging": map[string]any{"start": 0, "count": 2, "total": 2},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	creatives, err := ListCreatives(context.Background(), c, "42", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(creatives) != 2 {
		t.Fatalf("len: %d", len(creatives))
	}
	if creatives[0].ID != "urn:li:sponsoredCreative:1" {
		t.Errorf("id[0]: %q", creatives[0].ID)
	}
	if creatives[0].Review != "APPROVED" {
		t.Errorf("review: %q", creatives[0].Review)
	}
}

func TestGetCreative(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCreatives/urn:li:sponsoredCreative:1" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"urn:li:sponsoredCreative:1","status":"ACTIVE","intendedStatus":"ACTIVE","campaign":"urn:li:sponsoredCampaign:42"}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	cr, err := GetCreative(context.Background(), c, "urn:li:sponsoredCreative:1")
	if err != nil {
		t.Fatal(err)
	}
	if cr.ID != "urn:li:sponsoredCreative:1" {
		t.Errorf("id: %q", cr.ID)
	}
}
