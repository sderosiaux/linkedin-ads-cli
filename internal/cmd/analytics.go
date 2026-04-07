package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

const dateLayout = "2006-01-02"

func newAnalyticsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "analytics",
		Short: "Query ad analytics",
	}
	root.AddCommand(
		newAnalyticsCampaignsCmd(),
		newAnalyticsCreativesCmd(),
		newAnalyticsDemographicsCmd(),
		newAnalyticsReachCmd(),
		newAnalyticsDailyTrendsCmd(),
		newAnalyticsCompareCmd(),
	)
	return root
}

var demographicsPivots = map[string]struct{}{
	"JOB_FUNCTION":        {},
	"INDUSTRY":            {},
	"SENIORITY":           {},
	"COMPANY_SIZE":        {},
	"COUNTRY":             {},
	"REGION":              {},
	"MEMBER_JOB_FUNCTION": {},
	"MEMBER_SENIORITY":    {},
	"MEMBER_INDUSTRY":     {},
	"MEMBER_COMPANY_SIZE": {},
	"MEMBER_JOB_TITLE":    {},
	"MEMBER_COMPANY":      {},
	"MEMBER_COUNTRY":      {},
	"MEMBER_COUNTRY_V2":   {},
	"MEMBER_REGION":       {},
	"MEMBER_REGION_V2":    {},
}

// pivotAliases maps short-form demographic pivots to the canonical MEMBER_
// prefixed names that the LinkedIn API requires.
var pivotAliases = map[string]string{
	"JOB_FUNCTION": "MEMBER_JOB_FUNCTION",
	"SENIORITY":    "MEMBER_SENIORITY",
	"INDUSTRY":     "MEMBER_INDUSTRY",
	"COMPANY_SIZE": "MEMBER_COMPANY_SIZE",
	"COUNTRY":      "MEMBER_COUNTRY_V2",
	"REGION":       "MEMBER_REGION_V2",
	"JOB_TITLE":    "MEMBER_JOB_TITLE",
	"COMPANY":      "MEMBER_COMPANY",
}

// canonicalizePivot maps short aliases to the MEMBER_ prefixed form that
// LinkedIn requires. Already-canonical values pass through unchanged.
func canonicalizePivot(pivot string) string {
	if mapped, ok := pivotAliases[pivot]; ok {
		return mapped
	}
	return pivot
}

// limitAnalyticsRows truncates the rows slice to --limit when set. Call this
// before writeOutput so the terminal closure (which captures `rows`) also sees
// the limited slice. writeOutput's internal applyLimit handles JSON, but the
// terminal formatter closure captures the Go variable — not the `data` arg —
// so we must pre-truncate.
func limitAnalyticsRows(cmd *cobra.Command, rows []api.AnalyticsRow) []api.AnalyticsRow {
	if lim := limitFlag(cmd); lim > 0 && len(rows) > lim {
		return rows[:lim]
	}
	return rows
}

// parseDateRange reads --start / --end (YYYY-MM-DD). End defaults to today,
// start defaults to 30 days before end. Returns a clean error on bad input.
func parseDateRange(cmd *cobra.Command) (time.Time, time.Time, error) {
	startStr, _ := cmd.Flags().GetString("start")
	endStr, _ := cmd.Flags().GetString("end")
	now := time.Now().UTC()
	end := now
	if endStr != "" {
		t, err := time.Parse(dateLayout, endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid date: --end %q (want YYYY-MM-DD)", endStr)
		}
		end = t
	}
	start := end.AddDate(0, 0, -30)
	if startStr != "" {
		t, err := time.Parse(dateLayout, startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid date: --start %q (want YYYY-MM-DD)", startStr)
		}
		start = t
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, errors.New("invalid date range: --start is after --end")
	}
	return start, end, nil
}

func parseGranularity(cmd *cobra.Command) (string, error) {
	g, _ := cmd.Flags().GetString("granularity")
	if g == "" {
		return "ALL", nil
	}
	switch strings.ToUpper(g) {
	case "ALL", "DAILY", "MONTHLY":
		return strings.ToUpper(g), nil
	default:
		return "", fmt.Errorf("invalid --granularity %q (want DAILY, MONTHLY, or ALL)", g)
	}
}

func newAnalyticsCampaignsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaigns",
		Short: "Analytics rolled up by campaign for an account",
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
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}
			gran, err := parseGranularity(cmd)
			if err != nil {
				return err
			}
			rows, err := api.GetCampaignAnalytics(cmd.Context(), c, accountID, start, end, gran)
			if err != nil {
				return err
			}
			rows = limitAnalyticsRows(cmd, rows)
			return writeOutput(cmd, rows, func() string { return formatAnalyticsRows(rows) }, compactAnalyticsRow)
		},
	}
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	cmd.Flags().String("granularity", "ALL", "DAILY, MONTHLY, or ALL")
	return cmd
}

// pivotDisplay returns the best available pivot identifier for terminal output.
// LinkedIn returns pivotValues (array) in current API versions; legacy
// pivotValue (string) is kept as fallback.
func pivotDisplay(r api.AnalyticsRow) string {
	if len(r.PivotValues) > 0 {
		return r.PivotValues[0]
	}
	return r.PivotValue
}

// formatAnalyticsRows renders an AnalyticsRow slice as a small terminal table.
func formatAnalyticsRows(rows []api.AnalyticsRow) string {
	var b strings.Builder
	b.WriteString("PIVOT_VALUE                    IMPR    CLICKS  COST_USD  CONV  LEADS\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-30s %7d %7d %9s %5d %6d\n",
			truncate(pivotDisplay(r), 30), r.Impressions, r.Clicks, r.CostInUsd, r.Conversions, r.OneClickLeads)
	}
	return b.String()
}

func newAnalyticsCreativesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "creatives",
		Short: "Analytics rolled up by creative for a campaign",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			campaignID, _ := cmd.Flags().GetString("campaign")
			if campaignID == "" {
				return errors.New("--campaign required")
			}
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}
			gran, err := parseGranularity(cmd)
			if err != nil {
				return err
			}
			rows, err := api.GetCreativeAnalytics(cmd.Context(), c, campaignID, start, end, gran)
			if err != nil {
				return err
			}
			rows = limitAnalyticsRows(cmd, rows)
			return writeOutput(cmd, rows, func() string { return formatAnalyticsRows(rows) }, compactAnalyticsRow)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (required)")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	cmd.Flags().String("granularity", "ALL", "DAILY, MONTHLY, or ALL")
	return cmd
}

func newAnalyticsDemographicsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demographics",
		Short: "Demographic breakdown for a campaign",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			campaignID, _ := cmd.Flags().GetString("campaign")
			if campaignID == "" {
				return errors.New("--campaign required")
			}
			pivot, _ := cmd.Flags().GetString("pivot")
			pivot = strings.ToUpper(pivot)
			if _, ok := demographicsPivots[pivot]; !ok {
				return fmt.Errorf("invalid --pivot %q (want JOB_FUNCTION, INDUSTRY, SENIORITY, COMPANY_SIZE, COUNTRY, REGION, or MEMBER_* variants)", pivot)
			}
			pivot = canonicalizePivot(pivot)
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}
			rows, err := api.GetDemographicsAnalytics(cmd.Context(), c, campaignID, pivot, start, end)
			if err != nil {
				return err
			}
			rows = limitAnalyticsRows(cmd, rows)
			return writeOutput(cmd, rows, func() string { return formatAnalyticsRows(rows) }, compactAnalyticsRow)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (required)")
	cmd.Flags().String("pivot", "", "JOB_FUNCTION, INDUSTRY, SENIORITY, COMPANY_SIZE, COUNTRY, REGION, MEMBER_JOB_FUNCTION, MEMBER_SENIORITY, MEMBER_INDUSTRY, MEMBER_COMPANY_SIZE, MEMBER_JOB_TITLE, MEMBER_COMPANY, MEMBER_COUNTRY, MEMBER_COUNTRY_V2, MEMBER_REGION, MEMBER_REGION_V2")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}

func newAnalyticsReachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reach",
		Short: "Approximate unique reach for a campaign",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			campaignID, _ := cmd.Flags().GetString("campaign")
			if campaignID == "" {
				return errors.New("--campaign required")
			}
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}
			if end.Sub(start) > 92*24*time.Hour {
				return errors.New("reach queries are limited to 92 days")
			}
			rows, err := api.GetReachAnalytics(cmd.Context(), c, campaignID, start, end)
			if err != nil {
				return err
			}
			rows = limitAnalyticsRows(cmd, rows)
			return writeOutput(cmd, rows, func() string {
				var b strings.Builder
				b.WriteString("IMPR    MEMBER_REACH  AUDIENCE_PEN\n")
				for _, r := range rows {
					fmt.Fprintf(&b, "%7d %12d %12.6f\n", r.Impressions, r.MemberReach, r.AudiencePenetration)
				}
				return b.String()
			}, compactAnalyticsRow)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (required)")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}

func newAnalyticsDailyTrendsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily-trends",
		Short: "Daily timeseries scoped to a campaign or account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			campaignID, _ := cmd.Flags().GetString("campaign")
			accountID := ""
			if campaignID == "" {
				id, derr := accountIDFromFlagOrConfig(cmd, cfg)
				if derr != nil {
					return derr
				}
				accountID = id
			}
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}
			rows, err := api.GetDailyTrendsAnalytics(cmd.Context(), c, accountID, campaignID, start, end)
			if err != nil {
				return err
			}
			rows = limitAnalyticsRows(cmd, rows)
			return writeOutput(cmd, rows, func() string { return formatAnalyticsRows(rows) }, compactAnalyticsRow)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (overrides --account)")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}

// validCompareMetrics is the set accepted by `analytics compare --metric`.
var validCompareMetrics = map[string]struct{}{
	"spend":       {},
	"impressions": {},
	"clicks":      {},
	"ctr":         {},
	"cpc":         {},
}

func newAnalyticsCompareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two campaigns or campaign groups side-by-side",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, _ := cmd.Flags().GetString("a")
			b, _ := cmd.Flags().GetString("b")
			ga, _ := cmd.Flags().GetString("group-a")
			gb, _ := cmd.Flags().GetString("group-b")

			campaignMode := a != "" || b != ""
			groupMode := ga != "" || gb != ""
			if campaignMode && groupMode {
				return errors.New("--a/--b and --group-a/--group-b are mutually exclusive")
			}
			if !campaignMode && !groupMode {
				return errors.New("provide --a and --b (campaigns) or --group-a and --group-b (campaign groups)")
			}
			if campaignMode && (a == "" || b == "") {
				return errors.New("--a and --b campaign ids required")
			}
			if groupMode && (ga == "" || gb == "") {
				return errors.New("--group-a and --group-b campaign group ids required")
			}

			metric, _ := cmd.Flags().GetString("metric")
			metric = strings.ToLower(metric)
			if metric == "" {
				metric = "spend"
			}
			if _, ok := validCompareMetrics[metric]; !ok {
				return fmt.Errorf("invalid --metric %q (want spend, impressions, clicks, ctr, or cpc)", metric)
			}
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			start, end, err := parseDateRange(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			var (
				rowsA, rowsB   []api.AnalyticsRow
				errA, errB     error
				wg             sync.WaitGroup
				labelA, labelB string
			)
			wg.Add(2)
			if campaignMode {
				labelA, labelB = a, b
				go func() {
					defer wg.Done()
					rowsA, errA = api.GetSingleCampaignAnalytics(ctx, c, a, start, end)
				}()
				go func() {
					defer wg.Done()
					rowsB, errB = api.GetSingleCampaignAnalytics(ctx, c, b, start, end)
				}()
			} else {
				labelA, labelB = ga, gb
				go func() {
					defer wg.Done()
					rowsA, errA = api.GetSingleCampaignGroupAnalytics(ctx, c, ga, start, end)
				}()
				go func() {
					defer wg.Done()
					rowsB, errB = api.GetSingleCampaignGroupAnalytics(ctx, c, gb, start, end)
				}()
			}
			wg.Wait()
			if errA != nil {
				return fmt.Errorf("entity %s: %w", labelA, errA)
			}
			if errB != nil {
				return fmt.Errorf("entity %s: %w", labelB, errB)
			}
			return writeOutput(cmd, map[string]any{
				"a":      rowsA,
				"b":      rowsB,
				"metric": metric,
			}, func() string {
				return formatCompare(labelA, labelB, rowsA, rowsB, metric)
			})
		},
	}
	cmd.Flags().String("a", "", "First campaign id")
	cmd.Flags().String("b", "", "Second campaign id")
	cmd.Flags().String("group-a", "", "First campaign group id (mutually exclusive with --a/--b)")
	cmd.Flags().String("group-b", "", "Second campaign group id")
	cmd.Flags().String("metric", "spend", "spend, impressions, clicks, ctr, or cpc")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}

// firstRow returns the first analytics row or a zero value when the slice is empty.
func firstRow(rows []api.AnalyticsRow) api.AnalyticsRow {
	if len(rows) == 0 {
		return api.AnalyticsRow{}
	}
	return rows[0]
}

func metricValue(row api.AnalyticsRow, metric string) float64 {
	switch metric {
	case "impressions":
		return float64(row.Impressions)
	case "clicks":
		return float64(row.Clicks)
	case "spend":
		v, _ := strconv.ParseFloat(row.CostInUsd, 64)
		return v
	case "ctr":
		if row.Impressions == 0 {
			return 0
		}
		return float64(row.Clicks) / float64(row.Impressions)
	case "cpc":
		if row.Clicks == 0 {
			return 0
		}
		v, _ := strconv.ParseFloat(row.CostInUsd, 64)
		return v / float64(row.Clicks)
	}
	return 0
}

func formatCompare(aID, bID string, rowsA, rowsB []api.AnalyticsRow, metric string) string {
	a := firstRow(rowsA)
	b := firstRow(rowsB)
	var sb strings.Builder
	fmt.Fprintf(&sb, "                       A=%s        B=%s\n", aID, bID)
	fmt.Fprintf(&sb, "Impressions:           %12d  %12d\n", a.Impressions, b.Impressions)
	fmt.Fprintf(&sb, "Clicks:                %12d  %12d\n", a.Clicks, b.Clicks)
	fmt.Fprintf(&sb, "Cost (USD):            %12s  %12s\n", a.CostInUsd, b.CostInUsd)
	av := metricValue(a, metric)
	bv := metricValue(b, metric)
	delta := 0.0
	if av != 0 {
		delta = (bv - av) / av * 100
	}
	fmt.Fprintf(&sb, "%-22s %12.2f  %12.2f  Δ=%+.0f%%\n", "metric ("+metric+"):", av, bv, delta)
	return sb.String()
}
