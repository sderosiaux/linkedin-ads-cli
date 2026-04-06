package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestAnalyticsCampaigns_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CAMPAIGN") {
			t.Errorf("missing pivot: %s", raw)
		}
		if !strings.Contains(raw, "dateRange=(start:(year:2026,month:1,day:1),end:(year:2026,month:1,day:31))") {
			t.Errorf("dateRange shape wrong: %s", raw)
		}
		if !strings.Contains(raw, "accounts=List(urn:li:sponsoredAccount:777)") {
			t.Errorf("accounts list shape wrong: %s", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"impressions": 1000, "clicks": 50, "costInUsd": "12.34"},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--json",
		"analytics", "campaigns",
		"--start", "2026-01-01", "--end", "2026-01-31",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"impressions": 1000`) {
		t.Errorf("expected impressions, got: %s", out.String())
	}
}

func TestAnalyticsCampaigns_BadDate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "campaigns",
		"--start", "2026/01/01",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("expected 'invalid date' hint, got: %v", err)
	}
}
