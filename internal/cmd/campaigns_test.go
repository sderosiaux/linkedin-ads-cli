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
		if r.URL.Path != "/adAccounts/777/adCampaigns" {
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
			"metadata": map[string]any{},
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

func TestCampaignsList_EmptyState_Terminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[],"metadata":{}}`))
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
	root.SetArgs([]string{"--config", cfgPath, "campaigns", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "No campaigns in account 777") {
		t.Errorf("expected actionable empty-state hint, got: %s", s)
	}
	if !strings.Contains(s, "campaigns create") {
		t.Errorf("expected create hint, got: %s", s)
	}
	if strings.Contains(s, "STATUS") {
		t.Errorf("did not expect bare header row on empty list, got: %s", s)
	}
}

func TestCampaignsList_Compact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
					"dailyBudget":   map[string]any{"amount": "100", "currencyCode": "USD"},
				},
			},
			"metadata": map[string]any{},
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "--compact", "campaigns", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{`"id"`, `"campaignGroup"`, `"dailyBudget"`, `"objectiveType"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in compact whitelist, got: %s", want, s)
		}
	}
	for _, stripped := range []string{`"type"`, `"costType"`, `"account"`} {
		if strings.Contains(s, stripped) {
			t.Errorf("%s should be stripped in compact, got: %s", stripped, s)
		}
	}
}

func TestCampaignsBare_DelegatesToList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/777/adCampaigns" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":            55,
					"name":          "Bare",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
				},
			},
			"metadata": map[string]any{},
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
	// Bare `campaigns` with --status filter on the parent.
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "--status", "ACTIVE"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"id": 55`) {
		t.Errorf("expected id:55 in output: %s", out.String())
	}
}

func TestCampaignsList_GroupFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/777/adCampaigns" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("search.campaignGroup.values[0]") != "urn:li:sponsoredCampaignGroup:99" {
			t.Errorf("expected group filter, got q: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"elements":[],"metadata":{}}`))
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
		switch r.URL.Path {
		case "/adAccounts/777/adCampaigns":
			_, _ = w.Write([]byte(`{"elements":[
				{"id":10,"name":"C1","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"},
				{"id":11,"name":"C2","status":"PAUSED","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}
			],"metadata":{}}`))
		case "/adAccounts/777/adCampaignGroups/111":
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "list", "--resolve"})
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
	if !strings.Contains(out.String(), "POST /adAccounts/777/adCampaigns") {
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
	if gotMethod != http.MethodPost || gotPath != "/adAccounts/777/adCampaigns" {
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
		if r.Method == http.MethodGet && r.URL.Path == "/adAccounts/777/adCampaigns/10" {
			_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"PAUSED","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
			return
		}
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
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
	if gotMethod != http.MethodPost || gotPath != "/adAccounts/777/adCampaigns/10" {
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

func TestCampaignsUpdate_ShowsDiff_AndAppliesPatch(t *testing.T) {
	var gotPatch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adAccounts/777/adCampaigns/10" {
			_, _ = w.Write([]byte(`{"id":10,"name":"Old Name","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC","dailyBudget":{"amount":"50","currencyCode":"USD"}}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/adAccounts/777/adCampaigns/10" {
			gotPatch = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
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
		"campaigns", "update", "10",
		"--status", "PAUSED",
		"--name", "New Name",
		"--daily-budget", "100", "--currency", "USD",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !gotPatch {
		t.Errorf("expected PATCH call")
	}
	s := out.String()
	if !strings.Contains(s, "status: ACTIVE  →  PAUSED") {
		t.Errorf("expected status diff line, got: %s", s)
	}
	if !strings.Contains(s, "name: Old Name  →  New Name") {
		t.Errorf("expected name diff line, got: %s", s)
	}
	if !strings.Contains(s, "dailyBudget: 50 USD  →  100 USD") {
		t.Errorf("expected dailyBudget diff line, got: %s", s)
	}
	if !strings.Contains(s, "Updating campaign 10") {
		t.Errorf("expected diff header, got: %s", s)
	}
}

func TestCampaignsUpdate_NoChanges_ReturnsCleanly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adAccounts/777/adCampaigns/10" {
			_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
			return
		}
		if r.Method == http.MethodPost {
			t.Fatalf("PATCH should not be sent when nothing changed, got %s %s", r.Method, r.URL.Path)
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
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"campaigns", "update", "10", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.', got: %s", out.String())
	}
}

func TestCampaignsUpdate_DryRun_ShowsDiff_NoCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adAccounts/777/adCampaigns/10" {
			_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"PAUSED","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
			return
		}
		t.Fatalf("dry-run should not write, got %s %s", r.Method, r.URL.Path)
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
		"campaigns", "update", "10", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "POST /adAccounts/777/adCampaigns/10") {
		t.Errorf("expected summary in dry-run output: %s", out.String())
	}
	if !strings.Contains(out.String(), "status: PAUSED  →  ACTIVE") {
		t.Errorf("expected diff in dry-run output: %s", out.String())
	}
	if strings.Contains(out.String(), "correlation-id") {
		t.Errorf("dry-run should not emit correlation-id, got: %s", out.String())
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
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
	if gotMethod != http.MethodDelete || gotPath != "/adAccounts/777/adCampaigns/10" {
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
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
	if !strings.Contains(out.String(), "DELETE /adAccounts/777/adCampaigns/10") {
		t.Errorf("expected summary in dry-run output: %s", out.String())
	}
}

func TestCampaignsGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/777/adCampaigns/10" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"ACTIVE","account":"urn:li:sponsoredAccount:777","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "get", "10"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 10`) {
		t.Errorf("expected JSON id:10, got: %s", out.String())
	}
}
