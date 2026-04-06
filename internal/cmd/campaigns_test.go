package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestCampaignsList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaigns" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":            10,
					"name":          "Test",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
				},
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 10`) {
		t.Errorf("expected JSON id:10, got: %s", out.String())
	}
}

func TestCampaignsList_GroupFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		if !strings.Contains(search, "urn:li:sponsoredCampaignGroup:99") {
			t.Errorf("expected group filter, got: %q", search)
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
	root.SetArgs([]string{"--config", cfgPath, "campaigns", "list", "--group", "99"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignsList_ResolveJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/adCampaigns":
			_, _ = w.Write([]byte(`{"elements":[
				{"id":10,"name":"C1","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"},
				{"id":11,"name":"C2","status":"PAUSED","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}
			],"paging":{"start":0,"count":2,"total":2}}`))
		case strings.HasPrefix(r.URL.Path, "/adCampaignGroups/111"):
			_, _ = w.Write([]byte(`{"id":111,"name":"Q1 Push","status":"ACTIVE"}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "--resolve", "campaigns", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode envelope: %v\nbody: %s", err, out.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("expected 'data' key in envelope: %s", out.String())
	}
	resolved, ok := got["_resolved"].(map[string]any)
	if !ok {
		t.Fatalf("expected _resolved map, got: %s", out.String())
	}
	if resolved["urn:li:sponsoredCampaignGroup:111"] != "Q1 Push" {
		t.Errorf("expected resolved name 'Q1 Push', got: %v", resolved)
	}
}

func TestCampaignsCreate_DryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not call HTTP, got %s %s", r.Method, r.URL.Path)
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
		"--config", cfgPath, "--dry-run",
		"campaigns", "create",
		"--group", "678", "--name", "Spring",
		"--daily-budget", "100", "--currency", "USD",
		"--objective", "BRAND_AWARENESS", "--type", "SPONSORED_UPDATES",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "POST /adCampaigns") {
		t.Errorf("expected POST summary in dry-run output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"campaignGroup": "urn:li:sponsoredCampaignGroup:678"`) {
		t.Errorf("expected group URN in payload: %s", out.String())
	}
	if !strings.Contains(out.String(), `"name": "Spring"`) {
		t.Errorf("expected name in payload: %s", out.String())
	}
}

func TestCampaignsCreate_YesPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("X-LinkedIn-Id", "888")
		w.WriteHeader(http.StatusCreated)
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
		"--config", cfgPath, "--yes",
		"campaigns", "create",
		"--group", "678", "--name", "Spring",
		"--daily-budget", "100", "--currency", "USD",
		"--objective", "BRAND_AWARENESS", "--type", "SPONSORED_UPDATES",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaigns" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotBody["status"] != "DRAFT" || gotBody["costType"] != "CPM" {
		t.Errorf("body defaults: %+v", gotBody)
	}
	if !strings.Contains(out.String(), "Created campaign 888") {
		t.Errorf("expected success line, got: %s", out.String())
	}
}

func TestCampaignsUpdate_OnlyStatus(t *testing.T) {
	var gotMethod, gotPath, gotRestliMethod string
	var gotBodyRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRestliMethod = r.Header.Get("X-RestLi-Method")
		gotBodyRaw, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
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
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"campaigns", "update", "10", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaigns/10" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotRestliMethod != "PARTIAL_UPDATE" {
		t.Errorf("X-RestLi-Method: %q", gotRestliMethod)
	}
	expected := `{"patch":{"$set":{"status":"ACTIVE"}}}`
	if strings.TrimSpace(string(gotBodyRaw)) != expected {
		t.Errorf("body:\n got: %s\nwant: %s", string(gotBodyRaw), expected)
	}
}

func TestCampaignsDelete_YesPath(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
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
	root.SetArgs([]string{"--config", cfgPath, "--yes", "campaigns", "delete", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodDelete || gotPath != "/adCampaigns/10" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestCampaignsDelete_DryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not call HTTP, got %s %s", r.Method, r.URL.Path)
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
	root.SetArgs([]string{"--config", cfgPath, "--dry-run", "campaigns", "delete", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "DELETE /adCampaigns/10") {
		t.Errorf("expected summary in dry-run output: %s", out.String())
	}
}

func TestCampaignsGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaigns/10" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "get", "10"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 10`) {
		t.Errorf("expected JSON id:10, got: %s", out.String())
	}
}
