package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestGetLeadPerformance(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		for _, want := range []string{
			"q=analytics",
			"pivot=LEAD_GEN_FORM",
			"accounts=List(urn:li:sponsoredAccount:12345)",
		} {
			if !strings.Contains(raw, want) {
				t.Errorf("raw query missing %q in: %s", want, raw)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"pivotValue":       "urn:li:leadGenForm:1",
					"impressions":      1000,
					"clicks":           50,
					"oneClickLeads":    7,
					"leadGenFormOpens": 30,
					"costInUsd":        "12.34",
				},
			},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetLeadPerformance(context.Background(), c, "12345", "", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: %d", len(rows))
	}
	r := rows[0]
	if r.Form != "urn:li:leadGenForm:1" || r.Impressions != 1000 || r.LeadSubmissions != 7 {
		t.Errorf("row: %+v", r)
	}
}

func TestGetLeadPerformance_FormFilter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "leadGenForms=List(urn:li:leadGenForm:42)") {
			t.Errorf("missing leadGenForms filter in: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	if _, err := GetLeadPerformance(context.Background(), c, "12345", "42", start, end); err != nil {
		t.Fatal(err)
	}
}
