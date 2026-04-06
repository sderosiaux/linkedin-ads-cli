package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newAudiencesCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "audiences",
		Short: "List matched and lookalike audiences",
	}
	root.AddCommand(newAudiencesListCmd())
	return root
}

func newAudiencesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List DMP segments under an account",
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
			auds, err := api.ListAudiences(context.Background(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, auds, func() string {
				var b strings.Builder
				b.WriteString("ID         NAME                TYPE         STATUS    AUDIENCE  MATCHED\n")
				for _, a := range auds {
					fmt.Fprintf(&b, "%-10d %-19s %-12s %-9s %8d %8d\n",
						a.ID, truncate(a.Name, 19), truncate(a.Type, 12), a.Status, a.AudienceCount, a.MatchedCount)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	return cmd
}
