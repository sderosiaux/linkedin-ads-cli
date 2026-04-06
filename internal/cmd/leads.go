package cmd

import (
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newLeadsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "leads",
		Short: "Inspect lead-gen forms and submissions",
	}
	root.AddCommand(newLeadsFormsCmd())
	return root
}

func newLeadsFormsCmd() *cobra.Command {
	forms := &cobra.Command{
		Use:   "forms",
		Short: "Manage lead-gen forms",
	}
	forms.AddCommand(newLeadsFormsListCmd())
	return forms
}

func newLeadsFormsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List lead-gen forms under an account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			forms, err := api.ListLeadForms(cmd.Context(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, forms, func() string {
				var b strings.Builder
				b.WriteString("ID         NAME                STATUS    HEADLINE\n")
				for _, f := range forms {
					fmt.Fprintf(&b, "%-10d %-19s %-9s %s\n",
						f.ID, truncate(f.Name, 19), f.Status, truncate(f.Headline, 40))
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	return cmd
}
