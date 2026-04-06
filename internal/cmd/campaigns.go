package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/resolve"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
	"github.com/spf13/cobra"
)

func newCampaignsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "campaigns",
		Short: "List and inspect ad campaigns",
		Args:  cobra.NoArgs,
		RunE:  runCampaignsList,
	}
	addCampaignsListFlags(root)
	root.AddCommand(
		newCampaignsListCmd(),
		newCampaignsGetCmd(),
		newCampaignsCreateCmd(),
		newCampaignsUpdateCmd(),
		newCampaignsDeleteCmd(),
	)
	return root
}

func newCampaignsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaigns under an account (optionally filtered by group)",
		Args:  cobra.NoArgs,
		RunE:  runCampaignsList,
	}
	addCampaignsListFlags(cmd)
	return cmd
}

// addCampaignsListFlags wires the flags shared between `campaigns` (bare) and
// `campaigns list`. Both expose --account, --group, --status, --resolve.
func addCampaignsListFlags(cmd *cobra.Command) {
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("group", "", "Filter by campaign group id")
	cmd.Flags().String("status", "", "Filter by status (ACTIVE, DRAFT, ...)")
	cmd.Flags().Bool("resolve", false, "Enrich campaignGroup URNs with names (--json only)")
}

// runCampaignsList is shared by `campaigns` (bare) and `campaigns list`.
func runCampaignsList(cmd *cobra.Command, _ []string) error {
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
	camps, err := api.ListCampaigns(cmd.Context(), c, accountID, groupID, limitFlag(cmd))
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
	jsonOut, _ := cmd.Root().PersistentFlags().GetBool("json")
	var resolved map[string]string
	if jsonOut && resolveFlag(cmd) {
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
	}, compactCampaign)
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
			camp, err := api.GetCampaign(cmd.Context(), c, args[0])
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

func newCampaignsCreateCmd() *cobra.Command {
	var (
		groupID     string
		name        string
		dailyBudget int64
		objective   string
		typeFlag    string
		costType    string
		currency    string
		localeStr   string
		startStr    string
		endStr      string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new campaign",
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
			loc, err := parseLocale(localeStr)
			if err != nil {
				return err
			}
			in := &api.CreateCampaignInput{
				Account:       urn.Wrap(urn.Account, accountID),
				CampaignGroup: urn.Wrap(urn.CampaignGroup, groupID),
				Name:          name,
				Status:        "DRAFT",
				Type:          typeFlag,
				ObjectiveType: objective,
				CostType:      costType,
				Locale:        loc,
				DailyBudget: &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(dailyBudget, 10),
				},
			}
			if startStr != "" || endStr != "" {
				rs, err := parseDateRangeMillis(startStr, endStr)
				if err != nil {
					return err
				}
				in.RunSchedule = rs
			}
			return executeWrite(cmd, "POST /adCampaigns", in, func() error {
				id, err := api.CreateCampaign(cmd.Context(), c, in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created campaign %s\n", id)
				return err
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().StringVar(&groupID, "group", "", "Campaign group id (required)")
	cmd.Flags().StringVar(&name, "name", "", "Campaign name (required)")
	cmd.Flags().Int64Var(&dailyBudget, "daily-budget", 0, "Daily budget amount (required)")
	cmd.Flags().StringVar(&objective, "objective", "", "Objective (BRAND_AWARENESS, WEBSITE_VISIT, LEAD_GENERATION, ...) (required)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "Campaign type (SPONSORED_UPDATES, TEXT_AD, ...) (required)")
	cmd.Flags().StringVar(&costType, "cost-type", "CPM", "Cost type (CPM, CPC, CPV, ...)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code")
	cmd.Flags().StringVar(&localeStr, "locale", "en_US", "Locale, e.g. en_US")
	cmd.Flags().StringVar(&startStr, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endStr, "end", "", "End date YYYY-MM-DD")
	_ = cmd.MarkFlagRequired("group")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("daily-budget")
	_ = cmd.MarkFlagRequired("objective")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newCampaignsUpdateCmd() *cobra.Command {
	var (
		name        string
		status      string
		dailyBudget int64
		bid         int64
		currency    string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a campaign",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			in := &api.UpdateCampaignInput{}
			if cmd.Flags().Changed("name") {
				in.Name = &name
			}
			if cmd.Flags().Changed("status") {
				in.Status = &status
			}
			if cmd.Flags().Changed("daily-budget") {
				in.DailyBudget = &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(dailyBudget, 10),
				}
			}
			if cmd.Flags().Changed("bid") {
				in.UnitCost = &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(bid, 10),
				}
			}
			payload := map[string]any{
				"patch": map[string]any{"$set": in},
			}
			summary := "POST /adCampaigns/" + args[0]
			return executeWrite(cmd, summary, payload, func() error {
				if err := api.UpdateCampaign(cmd.Context(), c, args[0], in); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated campaign %s\n", args[0])
				return err
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&status, "status", "", "New status (DRAFT, ACTIVE, PAUSED, ARCHIVED, ...)")
	cmd.Flags().Int64Var(&dailyBudget, "daily-budget", 0, "New daily budget amount")
	cmd.Flags().Int64Var(&bid, "bid", 0, "New bid (unitCost) amount")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code (used with --daily-budget/--bid)")
	return cmd
}

func newCampaignsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a campaign",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			summary := "DELETE /adCampaigns/" + args[0]
			return executeWrite(cmd, summary, map[string]any{"id": args[0]}, func() error {
				if err := api.DeleteCampaign(cmd.Context(), c, args[0]); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted campaign %s\n", args[0])
				return err
			})
		},
	}
	return cmd
}

// parseLocale parses a "lang_COUNTRY" string (e.g. "en_US") into a Locale.
// Returns nil for an empty input.
func parseLocale(s string) (*api.Locale, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, "_")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid --locale %q (want lang_COUNTRY, e.g. en_US)", s)
	}
	return &api.Locale{Language: parts[0], Country: parts[1]}, nil
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
