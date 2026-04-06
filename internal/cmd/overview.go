package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

// overviewJSON is the structured shape rendered when --json is set. Spend is a
// float64 (parsed from LinkedIn's decimal-string costInUsd) so JSON consumers
// don't have to know about that quirk. The analytics_unavailable flag flips
// when the analytics call failed but resource counts still rendered.
type overviewJSON struct {
	Account              overviewAccountJSON `json:"account"`
	CampaignGroups       overviewCountsJSON  `json:"campaign_groups"`
	Campaigns            overviewCampJSON    `json:"campaigns"`
	SpendLast7d          float64             `json:"spend_last_7d"`
	ImpressionsLast7d    int64               `json:"impressions_last_7d"`
	ClicksLast7d         int64               `json:"clicks_last_7d"`
	AnalyticsUnavailable bool                `json:"analytics_unavailable,omitempty"`
}

type overviewAccountJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

type overviewCountsJSON struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

type overviewCampJSON struct {
	Active int `json:"active"`
	Paused int `json:"paused"`
	Total  int `json:"total"`
}

func newOverviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "One-screen account snapshot (groups, campaigns, last-7d spend)",
		Args:  cobra.NoArgs,
		RunE:  runOverview,
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	return cmd
}

func runOverview(cmd *cobra.Command, _ []string) error {
	c, cfg, err := clientFromConfig(cmd)
	if err != nil {
		return err
	}
	accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	acct, err := api.GetAccount(ctx, c, accountID)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7)

	var (
		groups    []api.CampaignGroup
		groupsErr error
		camps     []api.Campaign
		campsErr  error
		rows      []api.AnalyticsRow
		rowsErr   error
		wg        sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		groups, groupsErr = api.ListCampaignGroups(ctx, c, accountID, 0)
	}()
	go func() {
		defer wg.Done()
		camps, campsErr = api.ListCampaigns(ctx, c, accountID, "", 0)
	}()
	go func() {
		defer wg.Done()
		rows, rowsErr = api.GetCampaignAnalytics(ctx, c, accountID, start, end, "ALL")
	}()
	wg.Wait()

	if groupsErr != nil {
		return fmt.Errorf("list campaign groups: %w", groupsErr)
	}
	if campsErr != nil {
		return fmt.Errorf("list campaigns: %w", campsErr)
	}

	groupCounts := countCampaignGroups(groups)
	campCounts := countCampaigns(camps)
	spend, impressions, clicks := sumAnalytics(rows)
	analyticsUnavailable := rowsErr != nil

	data := overviewJSON{
		Account: overviewAccountJSON{
			ID:       acct.ID,
			Name:     acct.Name,
			Currency: acct.Currency,
		},
		CampaignGroups:       groupCounts,
		Campaigns:            campCounts,
		SpendLast7d:          spend,
		ImpressionsLast7d:    impressions,
		ClicksLast7d:         clicks,
		AnalyticsUnavailable: analyticsUnavailable,
	}

	return writeOutput(cmd, data, func() string {
		return formatOverview(data, analyticsUnavailable)
	})
}

func countCampaignGroups(groups []api.CampaignGroup) overviewCountsJSON {
	out := overviewCountsJSON{Total: len(groups)}
	for _, g := range groups {
		if g.Status == "ACTIVE" {
			out.Active++
		}
	}
	return out
}

func countCampaigns(camps []api.Campaign) overviewCampJSON {
	out := overviewCampJSON{Total: len(camps)}
	for _, c := range camps {
		switch c.Status {
		case "ACTIVE":
			out.Active++
		case "PAUSED":
			out.Paused++
		}
	}
	return out
}

// sumAnalytics rolls up cost (parsed from decimal string) and counters across
// all analytics rows. An empty CostInUsd contributes 0.
func sumAnalytics(rows []api.AnalyticsRow) (spend float64, impressions, clicks int64) {
	for _, r := range rows {
		if r.CostInUsd != "" {
			if v, err := strconv.ParseFloat(r.CostInUsd, 64); err == nil {
				spend += v
			}
		}
		impressions += r.Impressions
		clicks += r.Clicks
	}
	return
}

// formatMoney renders a float as "$1,234.50" — two decimals with US-style
// thousands separators. No external dep, just integer arithmetic on the
// pre-decimal portion.
func formatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(v)
	frac := int64((v-float64(whole))*100 + 0.5)
	if frac >= 100 {
		whole++
		frac = 0
	}
	wholeStr := strconv.FormatInt(whole, 10)
	// insert thousands separators every 3 digits from the right
	var b strings.Builder
	n := len(wholeStr)
	for i, r := range wholeStr {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s$%s.%02d", sign, b.String(), frac)
}

// formatInt renders an int64 with US-style thousands separators.
func formatInt(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	var b strings.Builder
	n := len(s)
	for i, r := range s {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func formatOverview(d overviewJSON, analyticsUnavailable bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Account:          %s (%d)\n", d.Account.Name, d.Account.ID)
	fmt.Fprintf(&b, "Currency:         %s\n", d.Account.Currency)
	b.WriteString("\n")
	fmt.Fprintf(&b, "Campaign Groups:  %d active / %d total\n", d.CampaignGroups.Active, d.CampaignGroups.Total)
	fmt.Fprintf(&b, "Campaigns:        %d active / %d paused / %d total\n", d.Campaigns.Active, d.Campaigns.Paused, d.Campaigns.Total)
	if analyticsUnavailable {
		b.WriteString("Spend (last 7d):  (unavailable)\n")
		b.WriteString("Impressions (7d): (unavailable)\n")
		b.WriteString("Clicks (7d):      (unavailable)\n")
	} else {
		fmt.Fprintf(&b, "Spend (last 7d):  %s\n", formatMoney(d.SpendLast7d))
		fmt.Fprintf(&b, "Impressions (7d): %s\n", formatInt(d.ImpressionsLast7d))
		fmt.Fprintf(&b, "Clicks (7d):      %s\n", formatInt(d.ClicksLast7d))
	}
	return b.String()
}
