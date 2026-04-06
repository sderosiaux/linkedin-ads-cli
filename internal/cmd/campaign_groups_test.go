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

func TestCampaignGroupsList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaignGroups" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Query().Get("search"), "urn:li:sponsoredAccount:777") {
			t.Errorf("search: %q", r.URL.Query().Get("search"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 111, "name": "Q1", "status": "ACTIVE", "account": "urn:li:sponsoredAccount:777"},
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaign-groups", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 111`) {
		t.Errorf("expected JSON id:111, got: %s", out.String())
	}
}

func TestCampaignGroupsList_AccountFlagOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("search"), "urn:li:sponsoredAccount:999") {
			t.Errorf("expected --account override 999, got search=%q", r.URL.Query().Get("search"))
		}
		_, _ = w.Write([]byte(`{"elements":[],"paging":{"start":0,"count":0,"total":0}}`))
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
	root.SetArgs([]string{"--config", cfgPath, "campaign-groups", "list", "--account", "999"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignGroupsList_NoAccountIsCleanError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "campaign-groups", "list"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no account") {
		t.Errorf("expected 'no account' hint, got: %v", err)
	}
}

func TestCampaignGroupsGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaignGroups/111" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":111,"name":"Q1","status":"ACTIVE","account":"urn:li:sponsoredAccount:777"}`))
	}))
	defer srv.Close()

	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaign-groups", "get", "111"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 111`) {
		t.Errorf("expected JSON id:111, got: %s", out.String())
	}
}
