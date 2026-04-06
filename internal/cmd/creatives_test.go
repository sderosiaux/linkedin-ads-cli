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

func TestCreativesList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/777/creatives" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             "urn:li:sponsoredCreative:1",
					"status":         "ACTIVE",
					"intendedStatus": "ACTIVE",
					"campaign":       "urn:li:sponsoredCampaign:42",
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "creatives", "list", "--campaign", "42"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": "urn:li:sponsoredCreative:1"`) {
		t.Errorf("expected JSON id, got: %s", out.String())
	}
}

func TestCreativesList_EmptyState_Terminal(t *testing.T) {
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
	root.SetArgs([]string{"--config", cfgPath, "creatives", "list", "--campaign", "42"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "No creatives on campaign 42") {
		t.Errorf("expected empty-state hint, got: %s", s)
	}
}

func TestCreativesList_Compact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             "urn:li:sponsoredCreative:1",
					"status":         "ACTIVE",
					"intendedStatus": "ACTIVE",
					"campaign":       "urn:li:sponsoredCampaign:42",
					"review":         map[string]any{"status": "APPROVED"},
					"createdAt":      1700000000000,
					"lastModifiedAt": 1710000000000,
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "--compact", "creatives", "list", "--campaign", "42"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{`"id"`, `"status"`, `"intendedStatus"`, `"campaign"`, `"review"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in compact whitelist, got: %s", want, s)
		}
	}
	for _, stripped := range []string{`"createdAt"`, `"lastModifiedAt"`} {
		if strings.Contains(s, stripped) {
			t.Errorf("%s should be stripped in compact, got: %s", stripped, s)
		}
	}
}

func TestCreativesList_MissingCampaignIsCleanError(t *testing.T) {
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
	root.SetArgs([]string{"--config", cfgPath, "creatives", "list"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--campaign") {
		t.Errorf("expected --campaign hint, got: %v", err)
	}
}

func TestCreativesCreate_DryRun(t *testing.T) {
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
		"creatives", "create",
		"--campaign", "42", "--content-reference", "urn:li:share:999",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "POST /adAccounts/777/creatives") {
		t.Errorf("expected POST summary, got: %s", s)
	}
	if !strings.Contains(s, "urn:li:sponsoredCampaign:42") {
		t.Errorf("expected campaign URN, got: %s", s)
	}
	if !strings.Contains(s, "urn:li:share:999") {
		t.Errorf("expected content reference, got: %s", s)
	}
}

func TestCreativesCreateInline_DryRun(t *testing.T) {
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
		"creatives", "create-inline",
		"--campaign", "42", "--org", "789", "--text", "Hello world",
		"--image", "urn:li:image:XXX", "--landing-page", "https://example.com",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "createInline") {
		t.Errorf("expected createInline in output, got: %s", s)
	}
	if !strings.Contains(s, "Hello world") {
		t.Errorf("expected commentary, got: %s", s)
	}
}

func TestCreativesUpdateStatus_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--dry-run", "creatives", "update-status", "123", "--status", "PAUSED"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "update-status") {
		t.Errorf("expected update-status in dry-run output, got: %s", s)
	}
	if !strings.Contains(s, "PAUSED") {
		t.Errorf("expected PAUSED in dry-run output, got: %s", s)
	}
}

func TestCreativesUpdateStatus_InvalidStatus(t *testing.T) {
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
	root.SetArgs([]string{"--config", cfgPath, "creatives", "update-status", "123", "--status", "INVALID"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid --status") {
		t.Errorf("expected status hint, got: %v", err)
	}
}

func TestCreativesGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept either encoded or decoded form.
		wantDecoded := "/adAccounts/777/creatives/urn:li:sponsoredCreative:1"
		if r.URL.Path != wantDecoded {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"urn:li:sponsoredCreative:1","status":"ACTIVE","intendedStatus":"ACTIVE","campaign":"urn:li:sponsoredCampaign:42"}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "creatives", "get", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": "urn:li:sponsoredCreative:1"`) {
		t.Errorf("expected JSON id, got: %s", out.String())
	}
}
