package cmd

import (
	"fmt"
	"sort"
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
	TopSpend             []overviewTopRow    `json:"top_spend,omitempty"`
	TopLeads             []overviewTopRow    `json:"top_leads,omitempty"`
	BudgetCap            float64             `json:"budget_cap,omitempty"`
	BudgetUtilization    float64             `json:"budget_utilization,omitempty"`
	ConversionTracking   string              `json:"conversion_tracking,omitempty"`
	AnalyticsUnavailable bool                `json:"analytics_unavailable,omitempty"`
}

// overviewTopRow is a top-N entry (by spend or by leads) with optional CPL
// for the leads variant.
type overviewTopRow struct {
	Name  string  `json:"name"`
	Spend float64 `json:"spend,omitempty"`
	Leads int64   `json:"leads,omitempty"`
	CPL   float64 `json:"cpl,omitempty"`
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

	if !analyticsUnavailable {
		// Top 3 by spend / leads (last 7d).
		data.TopSpend = top3CampaignsBySpend(camps, rows)
		data.TopLeads = top3CampaignsByLeads(camps, rows)
		// Budget utilization: sum of daily caps × 7 (since the analytics window
		// is 7 days) vs actual 7-day spend.
		var dailyCap float64
		for _, c := range camps {
			if c.Status == "ACTIVE" && c.DailyBudget != nil {
				if v, err := strconv.ParseFloat(c.DailyBudget.Amount, 64); err == nil {
					dailyCap += v
				}
			}
		}
		data.BudgetCap = dailyCap * 7
		if data.BudgetCap > 0 {
			data.BudgetUtilization = spend / data.BudgetCap * 100
		}
		// Conversion tracking status: BROKEN if every active row reports 0
		// conversions and 0 leads despite spend; OK otherwise.
		data.ConversionTracking = inferConversionTrackingStatus(rows)
	}

	return writeOutput(cmd, data, func() string {
		return formatOverview(data, analyticsUnavailable)
	})
}

// top3CampaignsBySpend ranks active campaigns by 7-day spend.
func top3CampaignsBySpend(camps []api.Campaign, rows []api.AnalyticsRow) []overviewTopRow {
	spendByID := map[int64]float64{}
	for _, r := range rows {
		urn := pivotDisplay(r)
		const prefix = "urn:li:sponsoredCampaign:"
		if !strings.HasPrefix(urn, prefix) {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(urn, prefix), 10, 64)
		if err != nil {
			continue
		}
		v, _ := strconv.ParseFloat(r.CostInUsd, 64)
		spendByID[id] += v
	}
	rowsOut := make([]overviewTopRow, 0, len(camps))
	for _, c := range camps {
		if c.Status != "ACTIVE" {
			continue
		}
		if v, ok := spendByID[c.ID]; ok && v > 0 {
			rowsOut = append(rowsOut, overviewTopRow{Name: c.Name, Spend: v})
		}
	}
	sort.Slice(rowsOut, func(i, j int) bool { return rowsOut[i].Spend > rowsOut[j].Spend })
	if len(rowsOut) > 3 {
		rowsOut = rowsOut[:3]
	}
	return rowsOut
}

// top3CampaignsByLeads ranks active campaigns by 7-day leads
// (oneClickLeads + externalWebsiteConversions). CPL is included where
// computable.
func top3CampaignsByLeads(camps []api.Campaign, rows []api.AnalyticsRow) []overviewTopRow {
	leadsByID := map[int64]int64{}
	spendByID := map[int64]float64{}
	for _, r := range rows {
		urn := pivotDisplay(r)
		const prefix = "urn:li:sponsoredCampaign:"
		if !strings.HasPrefix(urn, prefix) {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(urn, prefix), 10, 64)
		if err != nil {
			continue
		}
		leadsByID[id] += r.OneClickLeads + r.Conversions
		v, _ := strconv.ParseFloat(r.CostInUsd, 64)
		spendByID[id] += v
	}
	rowsOut := make([]overviewTopRow, 0, len(camps))
	for _, c := range camps {
		if c.Status != "ACTIVE" {
			continue
		}
		l := leadsByID[c.ID]
		if l == 0 {
			continue
		}
		row := overviewTopRow{Name: c.Name, Leads: l}
		if l > 0 {
			row.CPL = spendByID[c.ID] / float64(l)
		}
		rowsOut = append(rowsOut, row)
	}
	sort.Slice(rowsOut, func(i, j int) bool { return rowsOut[i].Leads > rowsOut[j].Leads })
	if len(rowsOut) > 3 {
		rowsOut = rowsOut[:3]
	}
	return rowsOut
}

// inferConversionTrackingStatus returns "OK", "WARN", or "BROKEN" based on
// the spend-vs-conversions ratio of the rows. BROKEN means rows report spend
// > 0 but every row reports 0 conversions and 0 leads.
func inferConversionTrackingStatus(rows []api.AnalyticsRow) string {
	var totalSpend float64
	var totalConv, totalLeads int64
	for _, r := range rows {
		v, _ := strconv.ParseFloat(r.CostInUsd, 64)
		totalSpend += v
		totalConv += r.Conversions
		totalLeads += r.OneClickLeads
	}
	if totalSpend == 0 {
		return "IDLE"
	}
	if totalConv == 0 && totalLeads == 0 {
		return "BROKEN"
	}
	return "OK"
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
	fmt.Fprintf(&b, "Account:             %s (%d)\n", d.Account.Name, d.Account.ID)
	fmt.Fprintf(&b, "Currency:            %s\n", d.Account.Currency)
	b.WriteString("\n")
	fmt.Fprintf(&b, "Campaign Groups:     %d active / %d total\n", d.CampaignGroups.Active, d.CampaignGroups.Total)
	fmt.Fprintf(&b, "Campaigns:           %d active / %d paused / %d total\n", d.Campaigns.Active, d.Campaigns.Paused, d.Campaigns.Total)
	if analyticsUnavailable {
		b.WriteString("\nLast 7d:\n  (unavailable)\n")
		return b.String()
	}
	b.WriteString("\nLast 7d:\n")
	fmt.Fprintf(&b, "  Spend:             %s\n", formatMoney(d.SpendLast7d))
	fmt.Fprintf(&b, "  Impressions:       %s\n", formatInt(d.ImpressionsLast7d))
	fmt.Fprintf(&b, "  Clicks:            %s\n", formatInt(d.ClicksLast7d))
	if len(d.TopSpend) > 0 {
		b.WriteString("\nTop 3 by spend:\n")
		for _, t := range d.TopSpend {
			fmt.Fprintf(&b, "  %-30s %s\n", truncate(t.Name, 30), formatMoney(t.Spend))
		}
	}
	if len(d.TopLeads) > 0 {
		b.WriteString("\nTop 3 by leads:\n")
		for _, t := range d.TopLeads {
			cpl := ""
			if t.CPL > 0 {
				cpl = fmt.Sprintf(" (%s CPL)", formatMoney(t.CPL))
			}
			fmt.Fprintf(&b, "  %-30s %d leads%s\n", truncate(t.Name, 30), t.Leads, cpl)
		}
	}
	if d.BudgetCap > 0 {
		fmt.Fprintf(&b, "\nBudget utilization:  %s / %s cap (%.0f%%)\n", formatMoney(d.SpendLast7d), formatMoney(d.BudgetCap), d.BudgetUtilization)
	}
	if d.ConversionTracking != "" {
		icon := "✓"
		switch d.ConversionTracking {
		case "BROKEN":
			icon = "⚠️"
		case "WARN":
			icon = "⚠️"
		}
		fmt.Fprintf(&b, "Conversion tracking: %s %s\n", icon, d.ConversionTracking)
	}
	return b.String()
}
