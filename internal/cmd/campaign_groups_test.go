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

func TestCampaignGroupsBare_DelegatesToList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaignGroups" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 222, "name": "Bare", "status": "ACTIVE", "account": "urn:li:sponsoredAccount:777"},
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaign-groups"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"id": 222`) {
		t.Errorf("expected id:222 in output: %s", out.String())
	}
}

func TestCampaignGroupsList_Compact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[
			{"id":111,"name":"Q1","status":"ACTIVE","account":"urn:li:sponsoredAccount:777",
			 "totalBudget":{"amount":"5000","currencyCode":"USD"},
			 "runSchedule":{"start":1700000000000,"end":1710000000000}}
		],"paging":{"start":0,"count":1,"total":1}}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "--compact", "campaign-groups", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{`"id"`, `"name"`, `"status"`, `"totalBudget"`, `"runSchedule"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in compact whitelist, got: %s", want, s)
		}
	}
	for _, stripped := range []string{`"account"`} {
		if strings.Contains(s, stripped) {
			t.Errorf("%s should be stripped in compact, got: %s", stripped, s)
		}
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

func TestCampaignGroupsList_ResolveJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/adCampaignGroups":
			_, _ = w.Write([]byte(`{"elements":[
				{"id":111,"name":"Q1","status":"ACTIVE","account":"urn:li:sponsoredAccount:777"}
			],"paging":{"start":0,"count":1,"total":1}}`))
		case strings.HasPrefix(r.URL.Path, "/adAccounts/777"):
			_, _ = w.Write([]byte(`{"id":777,"name":"Acme EMEA","currency":"USD"}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaign-groups", "list", "--resolve"})
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
	if resolved["urn:li:sponsoredAccount:777"] != "Acme EMEA" {
		t.Errorf("expected resolved name 'Acme EMEA', got: %v", resolved)
	}
}

func TestCampaignGroupsCreate_DryRun(t *testing.T) {
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
		"campaign-groups", "create",
		"--name", "Q2", "--total-budget", "5000", "--currency", "USD",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "POST /adCampaignGroups") {
		t.Errorf("expected POST summary in dry-run output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"name": "Q2"`) {
		t.Errorf("expected name in payload: %s", out.String())
	}
	if !strings.Contains(out.String(), `"account": "urn:li:sponsoredAccount:777"`) {
		t.Errorf("expected account URN in payload: %s", out.String())
	}
}

func TestCampaignGroupsCreate_YesPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("X-LinkedIn-Id", "999")
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
		"campaign-groups", "create",
		"--name", "Q2", "--total-budget", "5000", "--currency", "USD",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaignGroups" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotBody["name"] != "Q2" || gotBody["status"] != "DRAFT" {
		t.Errorf("body: %+v", gotBody)
	}
	if !strings.Contains(out.String(), "Created campaign group 999") {
		t.Errorf("expected success line, got: %s", out.String())
	}
}

func TestCampaignGroupsUpdate_OnlyStatus(t *testing.T) {
	var gotMethod, gotPath, gotRestliMethod string
	var gotBodyRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adCampaignGroups/111" {
			_, _ = w.Write([]byte(`{"id":111,"name":"Q1","status":"PAUSED","account":"urn:li:sponsoredAccount:777"}`))
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"campaign-groups", "update", "111", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaignGroups/111" {
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

func TestCampaignGroupsUpdate_ShowsDiff_AndAppliesPatch(t *testing.T) {
	var gotPatch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adCampaignGroups/12345" {
			_, _ = w.Write([]byte(`{"id":12345,"name":"Q2 Brand","status":"ACTIVE","account":"urn:li:sponsoredAccount:777"}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/adCampaignGroups/12345" {
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"campaign-groups", "update", "12345", "--status", "PAUSED",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !gotPatch {
		t.Errorf("expected PATCH call")
	}
	if !strings.Contains(out.String(), "status: ACTIVE  →  PAUSED") {
		t.Errorf("expected status diff line, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Updating campaign group 12345") {
		t.Errorf("expected diff header, got: %s", out.String())
	}
}

func TestCampaignGroupsUpdate_NoChanges_ReturnsCleanly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adCampaignGroups/12345" {
			_, _ = w.Write([]byte(`{"id":12345,"name":"Q2","status":"ACTIVE","account":"urn:li:sponsoredAccount:777"}`))
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
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"campaign-groups", "update", "12345", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.', got: %s", out.String())
	}
}

func TestCampaignGroupsUpdate_DryRun_ShowsDiff_NoCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/adCampaignGroups/111" {
			_, _ = w.Write([]byte(`{"id":111,"name":"Q1","status":"PAUSED","account":"urn:li:sponsoredAccount:777"}`))
			return
		}
		t.Fatalf("dry-run should not write, got %s %s", r.Method, r.URL.Path)
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
		"--config", cfgPath, "--dry-run",
		"campaign-groups", "update", "111", "--status", "ACTIVE",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "POST /adCampaignGroups/111") {
		t.Errorf("expected summary in dry-run output: %s", out.String())
	}
	if !strings.Contains(out.String(), "status: PAUSED  →  ACTIVE") {
		t.Errorf("expected diff in dry-run output: %s", out.String())
	}
	if strings.Contains(out.String(), "correlation-id") {
		t.Errorf("dry-run should not emit correlation-id, got: %s", out.String())
	}
}

func TestCampaignGroupsDelete_YesPath(t *testing.T) {
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
	root.SetArgs([]string{"--config", cfgPath, "--yes", "campaign-groups", "delete", "123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotMethod != http.MethodDelete || gotPath != "/adCampaignGroups/123" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestCampaignGroupsDelete_DryRun(t *testing.T) {
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
	root.SetArgs([]string{"--config", cfgPath, "--dry-run", "campaign-groups", "delete", "123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "DELETE /adCampaignGroups/123") {
		t.Errorf("expected summary in dry-run output: %s", out.String())
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
