package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestUseAccount_Persists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{Token: "x"}) //nolint:gosec // test fixture

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "use-account", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	c, _ := config.Load(cfgPath)
	if c.DefaultAccount != "12345" {
		t.Errorf("default_account: %q", c.DefaultAccount)
	}
	if c.Token != "x" {
		t.Errorf("token should be preserved: %q", c.Token)
	}
}

func TestCurrentAccount_None(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "current-account"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "(none)") {
		t.Errorf("expected '(none)', got: %s", out.String())
	}
}

func TestCurrentAccount_Set(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{DefaultAccount: "42"})

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "current-account"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "42") {
		t.Errorf("expected '42', got: %s", out.String())
	}
}
