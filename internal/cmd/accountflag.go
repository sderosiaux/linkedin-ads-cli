package cmd

import (
	"errors"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
)

// accountIDFromFlagOrConfig returns the --account flag value if set,
// otherwise the default account from config, otherwise an actionable error.
// Use this from any account-scoped command.
func accountIDFromFlagOrConfig(cmd *cobra.Command, cfg *config.Config) (string, error) {
	id, _ := cmd.Flags().GetString("account")
	if id != "" {
		return id, nil
	}
	if cfg != nil && cfg.DefaultAccount != "" {
		return cfg.DefaultAccount, nil
	}
	return "", errors.New("no account — pass --account <id> or run 'linkedin-ads use-account <id>'")
}
