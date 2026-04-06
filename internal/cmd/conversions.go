package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newConversionsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "conversions",
		Short: "List conversion definitions",
	}
	root.AddCommand(newConversionsListCmd())
	return root
}

func newConversionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversion definitions under an account",
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
			convs, err := api.ListConversions(context.Background(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, convs, func() string {
				var b strings.Builder
				b.WriteString("ID         NAME                TYPE         ENABLED  ATTRIBUTION\n")
				for _, c := range convs {
					fmt.Fprintf(&b, "%-10d %-19s %-12s %-8t %s\n",
						c.ID, truncate(c.Name, 19), truncate(c.Type, 12), c.Enabled, c.AttributionType)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	return cmd
}
