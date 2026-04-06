package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
)

const defaultBaseURL = "https://api.linkedin.com/rest"

// clientFromConfig loads the config referenced by the root --config flag and
// returns a ready-to-use HTTP client plus the loaded config.
//
// Returns an actionable error if no token has been saved. Defaults APIVersion
// to defaultAPIVersion when empty so commands always send a Linkedin-Version
// header.
//
// The LINKEDIN_ADS_BASE_URL environment variable overrides the base URL. This
// is a test-only hook and is intentionally not advertised in --help.
func clientFromConfig(cmd *cobra.Command) (*client.Client, *config.Config, error) {
	path := configPathFrom(cmd)
	if warning := config.CheckPerms(path); warning != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	c, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}
	if c.Token == "" {
		return nil, nil, errors.New("no token — run 'linkedin-ads auth login' first")
	}
	if c.APIVersion == "" {
		c.APIVersion = defaultAPIVersion
	}
	base := defaultBaseURL
	if v := os.Getenv("LINKEDIN_ADS_BASE_URL"); v != "" {
		base = v
	}
	return client.New(client.Options{
		BaseURL:    base,
		Token:      c.Token,
		APIVersion: c.APIVersion,
	}), c, nil
}
