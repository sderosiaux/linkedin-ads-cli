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
