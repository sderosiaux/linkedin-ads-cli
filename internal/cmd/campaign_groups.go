package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newCampaignGroupsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "campaign-groups",
		Short: "List and inspect ad campaign groups",
	}
	root.AddCommand(newCampaignGroupsListCmd(), newCampaignGroupsGetCmd())
	return root
}

func newCampaignGroupsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaign groups under an account",
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
			statusFilter, _ := cmd.Flags().GetString("status")
			groups, err := api.ListCampaignGroups(context.Background(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			if statusFilter != "" {
				filtered := groups[:0]
				for _, g := range groups {
					if strings.EqualFold(g.Status, statusFilter) {
						filtered = append(filtered, g)
					}
				}
				groups = filtered
			}
			return writeOutput(cmd, groups, func() string {
				var b strings.Builder
				b.WriteString("ID         NAME                STATUS    ACCOUNT\n")
				for _, g := range groups {
					fmt.Fprintf(&b, "%-10d %-19s %-9s %s\n",
						g.ID, truncate(g.Name, 19), g.Status, g.Account)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("status", "", "Filter by status (ACTIVE, DRAFT, ...)")
	return cmd
}

func newCampaignGroupsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single campaign group by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			g, err := api.GetCampaignGroup(context.Background(), c, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, g, func() string {
				return fmt.Sprintf("ID:      %d\nName:    %s\nStatus:  %s\nAccount: %s\n",
					g.ID, g.Name, g.Status, g.Account)
			})
		},
	}
}
