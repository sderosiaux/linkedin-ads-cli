package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestAuthLogin_FlagToken_WritesConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "auth", "login", "--token", "AQX_abc"})

	if err := root.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}

	c, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Token != "AQX_abc" {
		t.Errorf("token: %q", c.Token)
	}
	if c.APIVersion == "" {
		t.Errorf("APIVersion should be defaulted after login, got empty")
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perms: %v", info.Mode().Perm())
	}

	if !strings.Contains(out.String(), "Token saved") {
		t.Errorf("expected 'Token saved' in output, got: %s", out.String())
	}
}

func TestAuthLogout_ClearsToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{
		Token:          "abc",
		DefaultAccount: "12345",
		APIVersion:     "202601",
	})

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	c, _ := config.Load(cfgPath)
	if c.Token != "" {
		t.Errorf("token not cleared: %q", c.Token)
	}
	if c.DefaultAccount != "12345" {
		t.Errorf("default_account should be preserved, got: %q", c.DefaultAccount)
	}
	if c.APIVersion != "202601" {
		t.Errorf("api_version should be preserved, got: %q", c.APIVersion)
	}
}

func TestAuthStatus_NoToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "auth", "status"})
	_ = root.Execute()

	if !strings.Contains(out.String(), "not authenticated") {
		t.Errorf("expected 'not authenticated', got: %s", out.String())
	}
}

func TestAuthStatus_WithToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{Token: "AQX_longtokenstring_XYZ"}) //nolint:gosec // test fixture, not a real token

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	s := out.String()
	if !strings.Contains(s, "authenticated") {
		t.Errorf("expected 'authenticated' in output: %s", s)
	}
	// must NOT contain the full token
	if strings.Contains(s, "AQX_longtokenstring_XYZ") {
		t.Errorf("full token leaked in status output: %s", s)
	}
	// should show the last 4 chars masked (e.g., "...XYZ" or similar)
	if !strings.Contains(s, "XYZ") {
		t.Errorf("expected token tail hint in output: %s", s)
	}
}

func TestAuthLogin_PreservesExistingFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// pre-seed with account + version
	if err := config.Save(cfgPath, &config.Config{
		Token:          "old",
		DefaultAccount: "999",
		APIVersion:     "202501",
	}); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "auth", "login", "--token", "new"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	c, _ := config.Load(cfgPath)
	if c.Token != "new" {
		t.Errorf("token: %q", c.Token)
	}
	if c.DefaultAccount != "999" {
		t.Errorf("default_account lost: %q", c.DefaultAccount)
	}
	if c.APIVersion != "202501" {
		t.Errorf("api_version changed: %q", c.APIVersion)
	}
}
