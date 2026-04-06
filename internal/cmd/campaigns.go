package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/resolve"
	"github.com/spf13/cobra"
)

func newCampaignsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "campaigns",
		Short: "List and inspect ad campaigns",
	}
	root.AddCommand(newCampaignsListCmd(), newCampaignsGetCmd())
	return root
}

func newCampaignsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaigns under an account (optionally filtered by group)",
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
			groupID, _ := cmd.Flags().GetString("group")
			statusFilter, _ := cmd.Flags().GetString("status")
			camps, err := api.ListCampaigns(context.Background(), c, accountID, groupID, limitFlag(cmd))
			if err != nil {
				return err
			}
			if statusFilter != "" {
				filtered := camps[:0]
				for _, x := range camps {
					if strings.EqualFold(x.Status, statusFilter) {
						filtered = append(filtered, x)
					}
				}
				camps = filtered
			}
			var resolved map[string]string
			if resolveFlag(cmd) {
				urns := uniqueCampaignGroupURNs(camps)
				resolved = resolve.New(c).ResolveAll(cmd.Context(), urns)
			}
			return writeOutputWithResolved(cmd, camps, resolved, func() string {
				var b strings.Builder
				b.WriteString("ID         NAME                STATUS    TYPE                 OBJECTIVE          COST\n")
				for _, x := range camps {
					fmt.Fprintf(&b, "%-10d %-19s %-9s %-20s %-18s %s\n",
						x.ID, truncate(x.Name, 19), x.Status, truncate(x.Type, 20), truncate(x.Objective, 18), x.CostType)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("group", "", "Filter by campaign group id")
	cmd.Flags().String("status", "", "Filter by status (ACTIVE, DRAFT, ...)")
	return cmd
}

func newCampaignsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single campaign by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			camp, err := api.GetCampaign(context.Background(), c, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, camp, func() string {
				return fmt.Sprintf("ID:        %d\nName:      %s\nStatus:    %s\nType:      %s\nObjective: %s\nCostType:  %s\nGroup:     %s\nAccount:   %s\n",
					camp.ID, camp.Name, camp.Status, camp.Type, camp.Objective, camp.CostType, camp.CampaignGroup, camp.Account)
			})
		},
	}
}

// uniqueCampaignGroupURNs collects deduplicated, non-empty CampaignGroup URNs
// from a campaigns slice — used to feed Resolver.ResolveAll without doing
// redundant lookups.
func uniqueCampaignGroupURNs(camps []api.Campaign) []string {
	seen := make(map[string]struct{}, len(camps))
	out := make([]string, 0, len(camps))
	for _, c := range camps {
		if c.CampaignGroup == "" {
			continue
		}
		if _, ok := seen[c.CampaignGroup]; ok {
			continue
		}
		seen[c.CampaignGroup] = struct{}{}
		out = append(out, c.CampaignGroup)
	}
	return out
}
