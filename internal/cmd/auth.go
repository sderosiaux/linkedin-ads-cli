package cmd

import (
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
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
	auth.AddCommand(newAuthLoginCmd(), newAuthLogoutCmd(), newAuthStatusCmd())
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
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "✓ Token saved."); err != nil {
				return err
			}
			// Best-effort verification: list accessible accounts. The login is
			// not rolled back if this fails — the user may want to save a
			// token and verify later.
			client, _, err := clientFromConfig(cmd)
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not list accounts (%s). Token may be invalid; verify later with 'linkedin-ads accounts'.\n", err)
				return nil
			}
			accounts, err := api.ListAccounts(cmd.Context(), client, 0)
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not list accounts (%s). Token may be invalid; verify later with 'linkedin-ads accounts'.\n", err)
				return nil
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%d ad accounts accessible. Run 'linkedin-ads use-account <id>' to set a default.\n", len(accounts))
			return err
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "API token (skips interactive prompt)")
	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear the saved API token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			c.Token = ""
			if err := config.Save(path, c); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "✓ Token cleared.")
			return err
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			if c.Token == "" {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "not authenticated")
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "authenticated (token ...%s)\n", tail(c.Token, 4))
			return err
		},
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
