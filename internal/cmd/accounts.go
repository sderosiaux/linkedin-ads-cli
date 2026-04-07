package cmd

import (
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newAccountsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "accounts",
		Short: "List and inspect ad accounts",
		Args:  cobra.NoArgs,
		RunE:  runAccountsList,
	}
	root.AddCommand(newAccountsListCmd(), newAccountsGetCmd())
	return root
}

func newAccountsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List accessible ad accounts",
		Args:  cobra.NoArgs,
		RunE:  runAccountsList,
	}
}

// runAccountsList is shared by `accounts` (bare) and `accounts list` so users
// can list without typing the subcommand.
func runAccountsList(cmd *cobra.Command, _ []string) error {
	c, _, err := clientFromConfig(cmd)
	if err != nil {
		return err
	}
	limit := limitFlag(cmd)
	accts, err := api.ListAccounts(cmd.Context(), c, limit)
	if err != nil {
		return err
	}
	return writeOutput(cmd, accts, func() string {
		if len(accts) == 0 {
			return "No ad accounts accessible.\n"
		}
		var b strings.Builder
		b.WriteString("ID         NAME                STATUS   TYPE       CURRENCY\n")
		for _, a := range accts {
			fmt.Fprintf(&b, "%-10d %-19s %-8s %-10s %s\n",
				a.ID, truncate(a.Name, 19), a.Status, a.Type, a.Currency)
		}
		return b.String()
	}, compactAccount)
}

func newAccountsGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single ad account by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			if rawFlag(cmd) {
				return writeRawGet(cmd, c, "/adAccounts/"+args[0])
			}
			acct, err := api.GetAccount(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, acct, func() string {
				return fmt.Sprintf("ID:       %d\nName:     %s\nStatus:   %s\nType:     %s\nCurrency: %s\n",
					acct.ID, acct.Name, acct.Status, acct.Type, acct.Currency)
			})
		},
	}
	cmd.Flags().Bool("raw", false, "Dump the full raw API response as JSON (bypass typed decode)")
	return cmd
}

// truncate shortens s to at most n runes, appending an ellipsis when truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}
