package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	root.AddCommand(newAnalyticsCampaignsCmd())
	return root
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
			rows, err := api.GetCampaignAnalytics(context.Background(), c, accountID, start, end, gran)
			if err != nil {
				return err
			}
			return writeOutput(cmd, rows, func() string { return formatAnalyticsRows(rows) })
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	cmd.Flags().String("granularity", "ALL", "DAILY, MONTHLY, or ALL")
	return cmd
}

// formatAnalyticsRows renders an AnalyticsRow slice as a small terminal table.
func formatAnalyticsRows(rows []api.AnalyticsRow) string {
	var b strings.Builder
	b.WriteString("PIVOT_VALUE                    IMPR    CLICKS  COST_USD  CONV  LEADS\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-30s %7d %7d %9s %5d %6d\n",
			truncate(r.PivotValue, 30), r.Impressions, r.Clicks, r.CostInUsd, r.Conversions, r.OneClickLeads)
	}
	return b.String()
}
