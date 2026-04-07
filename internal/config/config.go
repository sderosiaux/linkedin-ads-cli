package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk state managed by the CLI.
type Config struct {
	Token          string `yaml:"token"`
	DefaultAccount string `yaml:"default_account"`
	APIVersion     string `yaml:"api_version"`
}

// Load reads the config file at path. If the file does not exist, returns a
// zero-value Config and no error.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is supplied by the CLI user by design
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config to path with 0600 perms, creating parent directories
// with 0700 perms as needed.
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// DefaultPath returns the default location of the config file:
// ~/.config/linkedin-ads/config.yaml. Always uses $HOME/.config (XDG convention
// for CLI tools) rather than os.UserConfigDir which returns ~/Library/Application
// Support on macOS — wrong for a terminal tool.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "linkedin-ads", "config.yaml")
	}
	return filepath.Join(home, ".config", "linkedin-ads", "config.yaml")
}

// CheckPerms returns a non-empty warning string when path exists and has any
// permission bits looser than 0600 (i.e. any group or world bits set).
// Returns "" when the file is missing, unreadable, or already 0600.
// Callers should print the warning to stderr without aborting.
func CheckPerms(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	perm := info.Mode().Perm()
	if perm&^0o600 == 0 {
		return ""
	}
	return fmt.Sprintf("warning: %s has permissions %#o (expected 0600). Fix with: chmod 600 %s", path, perm, path)
}
