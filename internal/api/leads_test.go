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
		if r.URL.Path != "/leadForms" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "q=owner") {
			t.Errorf("missing q=owner in: %s", raw)
		}
		// The compound owner param must have raw parens with encoded colons.
		if !strings.Contains(raw, "owner=(sponsoredAccount:urn%3Ali%3AsponsoredAccount%3A12345)") {
			t.Errorf("missing compound owner in: %s", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             1,
					"name":           "EBook download",
					"state":          "SUBMITTED",
					"owner":          map[string]any{"sponsoredAccount": "urn:li:sponsoredAccount:12345"},
					"creationLocale": map[string]any{"country": "US", "language": "en"},
					"versionId":      1,
					"created":        1700000000000,
					"lastModified":   1710000000000,
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
	if f.State != "SUBMITTED" {
		t.Errorf("state: %q", f.State)
	}
	if f.CreationLocale == nil || f.CreationLocale.Country != "US" {
		t.Errorf("creationLocale: %+v", f.CreationLocale)
	}
	if f.VersionID != 1 {
		t.Errorf("versionId: %d", f.VersionID)
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
			"accounts=List(urn%3Ali%3AsponsoredAccount%3A12345)",
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
		if !strings.Contains(raw, "leadGenForms=List(urn%3Ali%3AleadGenForm%3A42)") {
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
