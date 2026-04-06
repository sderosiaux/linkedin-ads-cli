package cmd

import (
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const defaultAPIVersion = "202601"

func newAuthCmd() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	auth.AddCommand(newAuthLoginCmd())
	return auth
}

func newAuthLoginCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save an API token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if token == "" {
				if _, err := fmt.Fprint(cmd.OutOrStdout(), "Token: "); err != nil {
					return err
				}
				b, err := term.ReadPassword(int(os.Stdin.Fd())) //nolint:gosec // stdin fd fits in int
				if err != nil {
					return fmt.Errorf("read token: %w", err)
				}
				token = string(b)
				if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
					return err
				}
			}
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			c.Token = token
			if c.APIVersion == "" {
				c.APIVersion = defaultAPIVersion
			}
			if err := config.Save(path, c); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "✓ Token saved.")
			return err
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "API token (skips interactive prompt)")
	return cmd
}
