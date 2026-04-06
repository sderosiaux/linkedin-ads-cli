package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestClientFromConfig_MissingTokenErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{}); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath}); err != nil {
		t.Fatal(err)
	}

	_, _, err := clientFromConfig(root)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error should guide user to 'auth login', got: %v", err)
	}
}

func TestClientFromConfig_ReturnsConfiguredClient(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{
		Token:          "AQX", //nolint:gosec // test fixture, not a real token
		DefaultAccount: "42",
		APIVersion:     "202601",
	}); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath}); err != nil {
		t.Fatal(err)
	}

	cli, cfg, err := clientFromConfig(root)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cli == nil {
		t.Fatal("nil client")
	}
	if cfg.DefaultAccount != "42" {
		t.Errorf("account: %q", cfg.DefaultAccount)
	}
}

func TestClientFromConfig_DefaultsAPIVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath}); err != nil {
		t.Fatal(err)
	}

	_, cfg, err := clientFromConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIVersion == "" {
		t.Error("APIVersion should be defaulted")
	}
}

func TestClientFromConfig_VersionDateOverride(t *testing.T) {
	// no t.Parallel — t.Setenv mutates process env
	var gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("Linkedin-Version")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath, "--version-date", "202605"}); err != nil {
		t.Fatal(err)
	}
	cli, _, err := clientFromConfig(root)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var out any
	if err := cli.GetJSON(context.Background(), "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if gotVersion != "202605" {
		t.Errorf("Linkedin-Version: got %q, want 202605", gotVersion)
	}
}

func TestClientFromConfig_VersionDateInvalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath, "--version-date", "2026-05"}); err != nil {
		t.Fatal(err)
	}
	_, _, err := clientFromConfig(root)
	if err == nil {
		t.Fatal("expected error for invalid --version-date")
	}
	if !strings.Contains(err.Error(), "YYYYMM") {
		t.Errorf("error should mention YYYYMM, got: %v", err)
	}
}

func TestClientFromConfig_BaseURLEnvOverride(t *testing.T) {
	// no t.Parallel — t.Setenv mutates process env
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	t.Setenv("LINKEDIN_ADS_BASE_URL", "http://example.invalid")

	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--config", cfgPath}); err != nil {
		t.Fatal(err)
	}

	cli, _, err := clientFromConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cli == nil {
		t.Fatal("nil client")
	}
}
