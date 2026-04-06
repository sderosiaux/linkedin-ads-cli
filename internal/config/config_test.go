package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCreatesFileWith0600(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	c := &Config{Token: "tok", DefaultAccount: "123", APIVersion: "202601"}
	if err := Save(path, c); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %v", info.Mode().Perm())
	}
}

func TestLoadRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	orig := &Config{Token: "tok", DefaultAccount: "123", APIVersion: "202601"}
	if err := Save(path, orig); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *orig {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", got, orig)
	}
}

func TestLoadMissingFileReturnsZero(t *testing.T) {
	t.Parallel()
	got, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Token != "" || got.DefaultAccount != "" {
		t.Fatalf("expected zero, got %+v", got)
	}
}

func TestSaveCreatesParentDirWith0700(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "linkedin-ads", "config.yaml")
	if err := Save(path, &Config{Token: "x"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("expected parent 0700, got %v", info.Mode().Perm())
	}
}
