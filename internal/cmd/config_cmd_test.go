package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestConfigShow_MasksToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{
		Token:          "AQX_supersecret_XYZ", //nolint:gosec // test fixture
		DefaultAccount: "12345",
		APIVersion:     "202601",
	})

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "config", "show"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	s := out.String()
	if strings.Contains(s, "AQX_supersecret_XYZ") {
		t.Errorf("token leaked: %s", s)
	}
	if !strings.Contains(s, "***") {
		t.Errorf("expected *** mask: %s", s)
	}
	if !strings.Contains(s, "12345") {
		t.Errorf("expected account in output: %s", s)
	}
	if !strings.Contains(s, "202601") {
		t.Errorf("expected api_version in output: %s", s)
	}
}

func TestConfigSetVersion_Persists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{Token: "x"}) //nolint:gosec // test fixture

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "config", "set-version", "202603"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	c, _ := config.Load(cfgPath)
	if c.APIVersion != "202603" {
		t.Errorf("version: %q", c.APIVersion)
	}
}

func TestConfigSetVersion_RejectsInvalidFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "config", "set-version", "not-a-date"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid version format")
	}
}
