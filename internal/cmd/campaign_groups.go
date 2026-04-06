package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/resolve"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
	"github.com/spf13/cobra"
)

func newCampaignGroupsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "campaign-groups",
		Short: "List and inspect ad campaign groups",
		Args:  cobra.NoArgs,
		RunE:  runCampaignGroupsList,
	}
	addCampaignGroupsListFlags(root)
	root.AddCommand(
		newCampaignGroupsListCmd(),
		newCampaignGroupsGetCmd(),
		newCampaignGroupsCreateCmd(),
		newCampaignGroupsUpdateCmd(),
		newCampaignGroupsDeleteCmd(),
	)
	return root
}

func newCampaignGroupsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaign groups under an account",
		Args:  cobra.NoArgs,
		RunE:  runCampaignGroupsList,
	}
	addCampaignGroupsListFlags(cmd)
	return cmd
}

// addCampaignGroupsListFlags wires the flags shared between
// `campaign-groups` (bare) and `campaign-groups list`.
func addCampaignGroupsListFlags(cmd *cobra.Command) {
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("status", "", "Filter by status (ACTIVE, DRAFT, ...)")
	cmd.Flags().Bool("resolve", false, "Enrich account URNs with names (--json only)")
}

// runCampaignGroupsList is shared by `campaign-groups` (bare) and
// `campaign-groups list`.
func runCampaignGroupsList(cmd *cobra.Command, _ []string) error {
	c, cfg, err := clientFromConfig(cmd)
	if err != nil {
		return err
	}
	accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
	if err != nil {
		return err
	}
	statusFilter, _ := cmd.Flags().GetString("status")
	groups, err := api.ListCampaignGroups(cmd.Context(), c, accountID, limitFlag(cmd))
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
	jsonOut, _ := cmd.Root().PersistentFlags().GetBool("json")
	var resolved map[string]string
	if jsonOut && resolveFlag(cmd) {
		urns := uniqueAccountURNs(groups)
		resolved = resolve.New(c).ResolveAll(cmd.Context(), urns)
	}
	return writeOutputWithResolved(cmd, groups, resolved, func() string {
		var b strings.Builder
		b.WriteString("ID         NAME                STATUS    ACCOUNT\n")
		for _, g := range groups {
			fmt.Fprintf(&b, "%-10d %-19s %-9s %s\n",
				g.ID, truncate(g.Name, 19), g.Status, g.Account)
		}
		return b.String()
	}, compactCampaignGroup)
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
			g, err := api.GetCampaignGroup(cmd.Context(), c, args[0])
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

func newCampaignGroupsCreateCmd() *cobra.Command {
	var (
		name        string
		totalBudget int64
		currency    string
		startStr    string
		endStr      string
		status      string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new campaign group",
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
			in := &api.CreateCampaignGroupInput{
				Account: urn.Wrap(urn.Account, accountID),
				Name:    name,
				Status:  status,
				TotalBudget: &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(totalBudget, 10),
				},
			}
			if in.Status == "" {
				in.Status = "DRAFT"
			}
			if startStr != "" || endStr != "" {
				rs, err := parseDateRangeMillis(startStr, endStr)
				if err != nil {
					return err
				}
				in.RunSchedule = rs
			}
			return executeWrite(cmd, "POST /adCampaignGroups", in, func() error {
				id, err := api.CreateCampaignGroup(cmd.Context(), c, in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created campaign group %s\n", id)
				return err
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().StringVar(&name, "name", "", "Campaign group name (required)")
	cmd.Flags().Int64Var(&totalBudget, "total-budget", 0, "Total budget amount (required)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code")
	cmd.Flags().StringVar(&startStr, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endStr, "end", "", "End date YYYY-MM-DD")
	cmd.Flags().StringVar(&status, "status", "DRAFT", "Initial status (DRAFT, ACTIVE, ...)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("total-budget")
	return cmd
}

func newCampaignGroupsUpdateCmd() *cobra.Command {
	var (
		name        string
		status      string
		totalBudget int64
		currency    string
		startStr    string
		endStr      string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a campaign group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			id := args[0]
			current, err := api.GetCampaignGroup(cmd.Context(), c, id)
			if err != nil {
				return err
			}

			in := &api.UpdateCampaignGroupInput{}
			diffs := []fieldDiff{}

			if cmd.Flags().Changed("status") && status != current.Status {
				s := status
				in.Status = &s
				diffs = append(diffs, fieldDiff{"status", current.Status, status})
			}
			if cmd.Flags().Changed("name") && name != current.Name {
				n := name
				in.Name = &n
				diffs = append(diffs, fieldDiff{"name", current.Name, name})
			}
			if cmd.Flags().Changed("total-budget") {
				newMoney := &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(totalBudget, 10),
				}
				oldStr := formatMoneyValue(current.TotalBudget)
				newStr := formatMoneyValue(newMoney)
				if oldStr != newStr {
					in.TotalBudget = newMoney
					diffs = append(diffs, fieldDiff{"totalBudget", oldStr, newStr})
				}
			}
			if startStr != "" || endStr != "" {
				rs, err := parseDateRangeMillis(startStr, endStr)
				if err != nil {
					return err
				}
				oldStart, oldEnd := dateRangeBounds(current.RunSchedule)
				newStart := formatEpochMillisDate(rs.Start)
				newEnd := formatEpochMillisDate(rs.End)
				changed := false
				if startStr != "" && newStart != oldStart {
					diffs = append(diffs, fieldDiff{"start", oldStart, newStart})
					changed = true
				}
				if endStr != "" && newEnd != oldEnd {
					diffs = append(diffs, fieldDiff{"end", oldEnd, newEnd})
					changed = true
				}
				if changed {
					in.RunSchedule = rs
				}
			}

			if len(diffs) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return err
			}

			header := fmt.Sprintf("Updating campaign group %s (%s)", id, current.Name)
			if err := printDiff(cmd, header, diffs); err != nil {
				return err
			}

			payload := map[string]any{
				"patch": map[string]any{"$set": in},
			}
			summary := "POST /adCampaignGroups/" + id
			return executeWrite(cmd, summary, payload, func() error {
				if err := api.UpdateCampaignGroup(cmd.Context(), c, id, in); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "✓ Updated.")
				return err
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&status, "status", "", "New status (DRAFT, ACTIVE, PAUSED, ARCHIVED, ...)")
	cmd.Flags().Int64Var(&totalBudget, "total-budget", 0, "New total budget amount")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code (used with --total-budget)")
	cmd.Flags().StringVar(&startStr, "start", "", "New start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endStr, "end", "", "New end date YYYY-MM-DD")
	return cmd
}

func newCampaignGroupsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a campaign group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			summary := "DELETE /adCampaignGroups/" + args[0]
			return executeWrite(cmd, summary, map[string]any{"id": args[0]}, func() error {
				if err := api.DeleteCampaignGroup(cmd.Context(), c, args[0]); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted campaign group %s\n", args[0])
				return err
			})
		},
	}
	return cmd
}

// parseDateRangeMillis converts YYYY-MM-DD --start/--end strings into a
// LinkedIn DateRange measured in epoch milliseconds. Either bound may be empty.
func parseDateRangeMillis(startStr, endStr string) (*api.DateRange, error) {
	rs := &api.DateRange{}
	if startStr != "" {
		t, err := time.Parse(dateLayout, startStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date: --start %q (want YYYY-MM-DD)", startStr)
		}
		rs.Start = t.UnixMilli()
	}
	if endStr != "" {
		t, err := time.Parse(dateLayout, endStr)
		if err != nil {
			return nil, fmt.Errorf("invalid date: --end %q (want YYYY-MM-DD)", endStr)
		}
		rs.End = t.UnixMilli()
	}
	return rs, nil
}

// uniqueAccountURNs collects deduplicated, non-empty Account URNs from a
// campaign-groups slice — used to feed Resolver.ResolveAll.
func uniqueAccountURNs(groups []api.CampaignGroup) []string {
	seen := make(map[string]struct{}, len(groups))
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		if g.Account == "" {
			continue
		}
		if _, ok := seen[g.Account]; ok {
			continue
		}
		seen[g.Account] = struct{}{}
		out = append(out, g.Account)
	}
	return out
}
