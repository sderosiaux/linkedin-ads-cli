package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
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
		newCampaignsTargetingCmd(),
		newCampaignsDiffCmd(),
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
// `campaigns list`. --account is a global persistent flag and is not declared
// here.
func addCampaignsListFlags(cmd *cobra.Command) {
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
	// When filtering client-side by status, fetch all rows first — the API
	// doesn't support server-side status filtering for campaigns.
	apiLimit := limitFlag(cmd)
	if statusFilter != "" {
		apiLimit = 0
	}
	camps, err := api.ListCampaigns(cmd.Context(), c, accountID, groupID, apiLimit)
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
	if lim := limitFlag(cmd); lim > 0 && len(camps) > lim {
		camps = camps[:lim]
	}
	jsonOut, _ := cmd.Root().PersistentFlags().GetBool("json")
	var resolved map[string]string
	if jsonOut && resolveFlag(cmd) {
		urns := uniqueCampaignGroupURNs(camps)
		r := resolve.New(c, accountID)
		if v, _ := cmd.Root().PersistentFlags().GetBool("verbose"); v {
			r.SetLogger(cmd.ErrOrStderr())
		}
		resolved = r.ResolveAll(cmd.Context(), urns)
	}
	return writeOutputWithResolved(cmd, camps, resolved, func() string {
		if len(camps) == 0 {
			return fmt.Sprintf("No campaigns in account %s.\nCreate one with: linkedin-ads campaigns create --group <id> --name ... --daily-budget ...\n", accountID)
		}
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
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single campaign by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			if rawFlag(cmd) {
				return writeRawGet(cmd, c, "/adAccounts/"+accountID+"/adCampaigns/"+args[0])
			}
			// Fetch the campaign and (in parallel) the last 30d analytics so the
			// terminal block can show pacing. JSON output stays minimal — the raw
			// API fields are what LLM consumers want.
			ctx := cmd.Context()
			var (
				camp    *api.Campaign
				campErr error
				rows    []api.AnalyticsRow
				rowsErr error
				wg      sync.WaitGroup
				end     = time.Now().UTC()
				start   = end.AddDate(0, 0, -30)
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				camp, campErr = api.GetCampaign(ctx, c, accountID, args[0])
			}()
			go func() {
				defer wg.Done()
				rows, rowsErr = api.GetSingleCampaignAnalytics(ctx, c, args[0], start, end)
			}()
			wg.Wait()
			if campErr != nil {
				return campErr
			}
			// Don't fail when analytics 403/empty — surface as missing pacing.
			_ = rowsErr
			return writeOutput(cmd, camp, func() string {
				return formatCampaignGet(camp, rows)
			})
		},
	}
	cmd.Flags().Bool("raw", false, "Dump the full raw API response as JSON (bypass typed decode)")
	return cmd
}

// formatCampaignGet renders the terminal block for campaigns get. Pacing
// fields (run duration, daily budget, last 30d spend, avg daily, serving) are
// shown when the analytics call succeeded.
func formatCampaignGet(camp *api.Campaign, rows []api.AnalyticsRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:             %d\nName:           %s\nStatus:         %s\nType:           %s\nObjective:      %s\nCostType:       %s\nGroup:          %s\nAccount:        %s\n",
		camp.ID, camp.Name, camp.Status, camp.Type, camp.Objective, camp.CostType, camp.CampaignGroup, camp.Account)

	if camp.TargetingCriteria != nil {
		inc := summarizeFacets(camp.TargetingCriteria.IncludedFacets())
		exc := summarizeFacets(camp.TargetingCriteria.ExcludedFacets())
		if inc != "" || exc != "" {
			b.WriteString("Targeting:\n")
			if inc != "" {
				fmt.Fprintf(&b, "  include: %s\n", inc)
			}
			if exc != "" {
				fmt.Fprintf(&b, "  exclude: %s\n", exc)
			}
		}
	}

	// Pacing block
	b.WriteString("\n")
	runDays := 0
	if camp.RunSchedule != nil && camp.RunSchedule.Start > 0 {
		startedAt := time.UnixMilli(camp.RunSchedule.Start).UTC()
		runDays = int(time.Since(startedAt).Hours() / 24)
		if runDays < 0 {
			runDays = 0
		}
		fmt.Fprintf(&b, "Run duration:   %d days (started %s)\n", runDays, startedAt.Format(dateLayout))
	}
	dailyBudget := 0.0
	if camp.DailyBudget != nil {
		if v, err := strconv.ParseFloat(camp.DailyBudget.Amount, 64); err == nil {
			dailyBudget = v
			fmt.Fprintf(&b, "Daily budget:   %s\n", formatMoney(v))
		}
	}
	var spend30d float64
	for _, r := range rows {
		if v, err := strconv.ParseFloat(r.CostInUsd, 64); err == nil {
			spend30d += v
		}
	}
	// Always show the spend line when we have a daily budget — a $0 row on a
	// paused campaign is itself a useful signal, and the next "Avg daily" line
	// would otherwise reference a number the reader can't see.
	if dailyBudget > 0 || spend30d > 0 || len(rows) > 0 {
		fmt.Fprintf(&b, "Last 30d spend: %s\n", formatMoney(spend30d))
	}
	if dailyBudget > 0 && (runDays > 0 || spend30d > 0) {
		denom := runDays
		if denom > 30 || denom == 0 {
			denom = 30
		}
		avg := spend30d / float64(denom)
		pct := 0.0
		if dailyBudget > 0 {
			pct = avg / dailyBudget * 100
		}
		fmt.Fprintf(&b, "Avg daily:      %s (%.0f%% of cap)\n", formatMoney(avg), pct)
	}
	if len(camp.ServingStatuses) > 0 {
		fmt.Fprintf(&b, "Serving:        %s\n", strings.Join(camp.ServingStatuses, ", "))
	}
	return b.String()
}

// newCampaignsTargetingCmd prints a campaign's targeting criteria — either the
// raw TargetingCriteria struct as JSON, or a facet-by-facet terminal breakdown
// with optional URN resolution. Accepts one or more campaign ids, or fetches
// them via --all-active / --group <id>.
func newCampaignsTargetingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "targeting [id...]",
		Short: "Show targeting criteria for one or more campaigns",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			allActive, _ := cmd.Flags().GetBool("all-active")
			groupID, _ := cmd.Flags().GetString("group")

			modes := 0
			if len(args) > 0 {
				modes++
			}
			if allActive {
				modes++
			}
			if groupID != "" {
				modes++
			}
			if modes == 0 {
				return errors.New("provide at least one campaign id, --all-active, or --group <id>")
			}
			if modes > 1 {
				return errors.New("positional ids, --all-active, and --group are mutually exclusive")
			}

			ids, err := resolveTargetingCampaignIDs(cmd, c, accountID, args, allActive, groupID)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No campaigns matched.")
				return err
			}

			camps := make([]*api.Campaign, 0, len(ids))
			for _, id := range ids {
				camp, gerr := api.GetCampaign(cmd.Context(), c, accountID, id)
				if gerr != nil {
					return fmt.Errorf("get campaign %s: %w", id, gerr)
				}
				camps = append(camps, camp)
			}

			var resolver *resolve.Resolver
			if resolveFlag(cmd) {
				resolver = resolve.New(c, accountID)
			}

			jsonOut, _ := cmd.Root().PersistentFlags().GetBool("json")
			if jsonOut {
				if len(camps) == 1 {
					return writeOutput(cmd, camps[0].TargetingCriteria, func() string { return "" })
				}
				payload := make([]map[string]any, 0, len(camps))
				for _, c := range camps {
					payload = append(payload, map[string]any{
						"id":                c.ID,
						"name":              c.Name,
						"targetingCriteria": c.TargetingCriteria,
					})
				}
				return writeOutput(cmd, payload, func() string { return "" })
			}

			var b strings.Builder
			for i, c := range camps {
				if len(camps) > 1 {
					if i > 0 {
						b.WriteString("\n")
					}
					fmt.Fprintf(&b, "━━━ %s (%d) ━━━\n", c.Name, c.ID)
				}
				b.WriteString(formatTargeting(cmd, c, resolver))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
	cmd.Flags().Bool("resolve", false, "Resolve facet URNs to human-readable names")
	cmd.Flags().Bool("all-active", false, "Dump targeting for every ACTIVE campaign in the account")
	cmd.Flags().String("group", "", "Dump targeting for every campaign in the given campaign group id")
	return cmd
}

// resolveTargetingCampaignIDs picks the campaign id list to operate on for
// `campaigns targeting`. Exactly one of args / allActive / groupID is set by
// the time this is called (the caller validates mutual exclusivity).
func resolveTargetingCampaignIDs(cmd *cobra.Command, c *client.Client, accountID string, args []string, allActive bool, groupID string) ([]string, error) {
	switch {
	case len(args) > 0:
		return args, nil
	case allActive:
		camps, err := api.ListCampaigns(cmd.Context(), c, accountID, "", 0)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(camps))
		for _, x := range camps {
			if strings.EqualFold(x.Status, "ACTIVE") {
				out = append(out, strconv.FormatInt(x.ID, 10))
			}
		}
		return out, nil
	case groupID != "":
		camps, err := api.ListCampaigns(cmd.Context(), c, accountID, groupID, 0)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(camps))
		for _, x := range camps {
			out = append(out, strconv.FormatInt(x.ID, 10))
		}
		return out, nil
	}
	return nil, nil
}

// summarizeFacets renders a facet→values map as a compact comma-separated
// "facet(n)" string sorted by facet name. Returns "" on empty input. Facet
// URN prefixes are stripped so urn:li:adTargetingFacet:titles becomes titles.
func summarizeFacets(facets map[string][]string) string {
	if len(facets) == 0 {
		return ""
	}
	keys := make([]string, 0, len(facets))
	for k := range facets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s(%d)", shortFacetName(k), len(facets[k])))
	}
	return strings.Join(parts, ", ")
}

// shortFacetName strips the urn:li:adTargetingFacet: prefix from a facet URN.
func shortFacetName(facet string) string {
	const prefix = "urn:li:adTargetingFacet:"
	return strings.TrimPrefix(facet, prefix)
}

// formatTargeting renders a campaign's TargetingCriteria as a human-readable
// block. When resolver is non-nil, URNs are annotated with their resolved name
// after an em-dash.
func formatTargeting(cmd *cobra.Command, camp *api.Campaign, resolver *resolve.Resolver) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Targeting for %s (%d)\n", camp.Name, camp.ID)
	if camp.TargetingCriteria == nil {
		b.WriteString("\n(no targeting criteria)\n")
		return b.String()
	}
	inc := camp.TargetingCriteria.IncludedFacets()
	exc := camp.TargetingCriteria.ExcludedFacets()
	if len(inc) == 0 && len(exc) == 0 {
		b.WriteString("\n(empty targeting criteria)\n")
		return b.String()
	}
	if len(inc) > 0 {
		b.WriteString("\nINCLUDE:\n")
		writeFacets(cmd, &b, inc, resolver)
	}
	if len(exc) > 0 {
		b.WriteString("\nEXCLUDE:\n")
		writeFacets(cmd, &b, exc, resolver)
	}
	return b.String()
}

// writeFacets writes the facet→values body of INCLUDE/EXCLUDE sections, sorted
// by facet name for stable output. Resolved names (when resolver is non-nil)
// are appended after an em-dash on each value line.
func writeFacets(cmd *cobra.Command, b *strings.Builder, facets map[string][]string, resolver *resolve.Resolver) {
	keys := make([]string, 0, len(facets))
	for k := range facets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals := facets[k]
		fmt.Fprintf(b, "  %s (%d)\n", shortFacetName(k), len(vals))
		for _, v := range vals {
			if resolver != nil {
				name := resolver.Resolve(cmd.Context(), v)
				if name != "" && name != v {
					fmt.Fprintf(b, "    %s — %s\n", v, name)
					continue
				}
			}
			fmt.Fprintf(b, "    %s\n", v)
		}
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
			summary := fmt.Sprintf("POST /adAccounts/%s/adCampaigns", accountID)
			return executeWrite(cmd, summary, in, func() error {
				id, err := api.CreateCampaign(cmd.Context(), c, accountID, in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created campaign %s\n", id)
				return err
			})
		},
	}
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
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			id := args[0]
			current, err := api.GetCampaign(cmd.Context(), c, accountID, id)
			if err != nil {
				return err
			}

			in := &api.UpdateCampaignInput{}
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
			if cmd.Flags().Changed("daily-budget") {
				newMoney := &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(dailyBudget, 10),
				}
				oldStr := formatMoneyValue(current.DailyBudget)
				newStr := formatMoneyValue(newMoney)
				if oldStr != newStr {
					in.DailyBudget = newMoney
					diffs = append(diffs, fieldDiff{"dailyBudget", oldStr, newStr})
				}
			}
			if cmd.Flags().Changed("bid") {
				newMoney := &api.Money{
					CurrencyCode: currency,
					Amount:       strconv.FormatInt(bid, 10),
				}
				oldStr := formatMoneyValue(current.UnitCost)
				newStr := formatMoneyValue(newMoney)
				if oldStr != newStr {
					in.UnitCost = newMoney
					diffs = append(diffs, fieldDiff{"bid", oldStr, newStr})
				}
			}

			if len(diffs) == 0 {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
				return err
			}

			header := fmt.Sprintf("Updating campaign %s (%s)", id, current.Name)
			if err := printDiff(cmd, header, diffs); err != nil {
				return err
			}

			payload := map[string]any{
				"patch": map[string]any{"$set": in},
			}
			summary := fmt.Sprintf("POST /adAccounts/%s/adCampaigns/%s", accountID, id)
			return executeWrite(cmd, summary, payload, func() error {
				if err := api.UpdateCampaign(cmd.Context(), c, accountID, id, in); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "✓ Updated.")
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
		Short: "Delete a campaign (DRAFT: hard-delete; otherwise: set PENDING_DELETION)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			id := args[0]

			// Fetch current state to decide hard-delete vs soft-delete.
			current, err := api.GetCampaign(cmd.Context(), c, accountID, id)
			if err != nil {
				return err
			}
			isDraft := current.Status == "DRAFT"

			if isDraft {
				summary := fmt.Sprintf("DELETE /adAccounts/%s/adCampaigns/%s", accountID, id)
				return executeWrite(cmd, summary, map[string]any{"id": id}, func() error {
					if err := api.DeleteCampaign(cmd.Context(), c, accountID, id); err != nil {
						return err
					}
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted campaign %s\n", id)
					return err
				})
			}

			status := "PENDING_DELETION"
			in := &api.UpdateCampaignInput{Status: &status}
			payload := map[string]any{"patch": map[string]any{"$set": in}}
			summary := fmt.Sprintf("POST /adAccounts/%s/adCampaigns/%s (soft-delete)", accountID, id)
			return executeWrite(cmd, summary, payload, func() error {
				if err := api.UpdateCampaign(cmd.Context(), c, accountID, id, in); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Campaign %s set to PENDING_DELETION (non-draft cannot be hard-deleted)\n", id)
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
