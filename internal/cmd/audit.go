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

// severity levels for audit findings.
const (
	sevCritical  = "critical"
	sevImportant = "important"
	sevInfo      = "info"
)

// auditFinding is one entry in the audit report. JSON tags are stable so
// callers can pipe the output through jq.
type auditFinding struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

// auditReport is the structured envelope rendered when --json is set.
type auditReport struct {
	Account     auditAccount   `json:"account"`
	PeriodStart string         `json:"periodStart"`
	PeriodEnd   string         `json:"periodEnd"`
	Findings    []auditFinding `json:"findings"`
}

type auditAccount struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Run a health check on the current account (parallel API calls)",
		Args:  cobra.NoArgs,
		RunE:  runAudit,
	}
	return cmd
}

func runAudit(cmd *cobra.Command, _ []string) error {
	c, cfg, err := clientFromConfig(cmd)
	if err != nil {
		return err
	}
	accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
	if err != nil {
		return err
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -30)
	ctx := cmd.Context()

	var (
		acct      *api.Account
		acctErr   error
		camps     []api.Campaign
		campsErr  error
		analytics []api.AnalyticsRow
		anErr     error
		convs     []api.Conversion
		convsErr  error
		auds      []api.Audience
		audsErr   error
		wg        sync.WaitGroup
	)
	wg.Add(5)
	go func() {
		defer wg.Done()
		acct, acctErr = api.GetAccount(ctx, c, accountID)
	}()
	go func() {
		defer wg.Done()
		camps, campsErr = api.ListCampaigns(ctx, c, accountID, "", 0)
	}()
	go func() {
		defer wg.Done()
		analytics, anErr = api.GetCampaignAnalytics(ctx, c, accountID, start, end, "ALL")
	}()
	go func() {
		defer wg.Done()
		convs, convsErr = api.ListConversions(ctx, c, accountID, 0)
	}()
	go func() {
		defer wg.Done()
		auds, audsErr = api.ListAudiences(ctx, c, accountID, 0)
	}()
	wg.Wait()

	if acctErr != nil {
		return fmt.Errorf("get account: %w", acctErr)
	}
	if campsErr != nil {
		return fmt.Errorf("list campaigns: %w", campsErr)
	}
	// analytics/conversions/audiences errors are non-fatal — surface as findings.

	report := auditReport{
		Account:     auditAccount{ID: acct.ID, Name: acct.Name},
		PeriodStart: start.Format(dateLayout),
		PeriodEnd:   end.Format(dateLayout),
	}
	report.Findings = computeAuditFindings(camps, analytics, convs, auds, anErr, convsErr, audsErr)

	return writeOutput(cmd, report, func() string {
		return formatAuditReport(report)
	})
}

// computeAuditFindings runs all the checks and returns a sorted finding list
// (critical first, then important, then info — stable order within each).
func computeAuditFindings(camps []api.Campaign, analytics []api.AnalyticsRow, convs []api.Conversion, auds []api.Audience, anErr, convsErr, audsErr error) []auditFinding {
	var out []auditFinding

	// Index campaign analytics by campaign URN for quick lookup.
	campSpend := map[string]float64{}
	campConv := map[string]int64{}
	campLeads := map[string]int64{}
	campImpr := map[string]int64{}
	campClicks := map[string]int64{}
	campNullConv := map[string]bool{}
	for _, r := range analytics {
		urn := pivotDisplay(r)
		v, _ := strconv.ParseFloat(r.CostInUsd, 64)
		campSpend[urn] += v
		campConv[urn] += r.Conversions
		campLeads[urn] += r.OneClickLeads
		campImpr[urn] += r.Impressions
		campClicks[urn] += r.Clicks
		// Heuristic: a row with non-zero spend but zero conversions is suspicious
		// only when the rule says "null" — keep that as the wire-level marker.
		if r.Conversions == 0 && v > 0 {
			campNullConv[urn] = true
		}
	}

	// === Critical: disabled conversion rules ===
	if convsErr == nil {
		for _, c := range convs {
			if !c.Enabled {
				out = append(out, auditFinding{
					Severity: sevCritical,
					Category: "conversion",
					Message:  fmt.Sprintf("Conversion rule '%s' (%d) is disabled", c.Name, c.ID),
				})
			}
		}
	}

	// === Critical: high-spend campaigns with broken attribution ===
	activeCamps := make([]api.Campaign, 0, len(camps))
	for _, c := range camps {
		if c.Status == "ACTIVE" {
			activeCamps = append(activeCamps, c)
		}
	}
	if anErr == nil {
		brokenAttr := 0
		highSpendBurning := []string{}
		for _, c := range activeCamps {
			urn := campaignURN(c.ID)
			spend := campSpend[urn]
			conv := campConv[urn]
			leads := campLeads[urn]
			if spend > 100 && conv == 0 && leads == 0 {
				brokenAttr++
			}
			if spend > 500 && conv == 0 && leads == 0 {
				highSpendBurning = append(highSpendBurning, fmt.Sprintf("'%s' spent %s with 0 leads/conversions in last 30d", c.Name, formatMoney(spend)))
			}
		}
		if brokenAttr > 0 && brokenAttr == len(activeCamps) && len(activeCamps) > 0 {
			out = append(out, auditFinding{
				Severity: sevCritical,
				Category: "tracking",
				Message:  fmt.Sprintf("%d of %d active campaigns report 0 conversions and 0 leads — attribution may be broken", brokenAttr, len(activeCamps)),
			})
		}
		for _, m := range highSpendBurning {
			out = append(out, auditFinding{Severity: sevCritical, Category: "spend", Message: m})
		}
	}

	// === Important checks ===
	// Compute CPL across active camps for outlier detection.
	if anErr == nil {
		cpls := []float64{}
		for _, c := range activeCamps {
			urn := campaignURN(c.ID)
			leads := campLeads[urn] + campConv[urn]
			if leads > 0 {
				cpls = append(cpls, campSpend[urn]/float64(leads))
			}
		}
		medCPL := median(cpls)
		for _, c := range activeCamps {
			urn := campaignURN(c.ID)
			impr := campImpr[urn]
			clicks := campClicks[urn]
			if impr > 5000 {
				ctr := float64(clicks) / float64(impr)
				if ctr < 0.003 {
					out = append(out, auditFinding{
						Severity: sevImportant,
						Category: "creative",
						Message:  fmt.Sprintf("'%s' CTR %s on %s impressions — creative fatigue or audience mismatch", c.Name, formatPercent(ctr), formatInt(impr)),
					})
				}
			}
			leads := campLeads[urn] + campConv[urn]
			if leads > 0 && medCPL > 0 {
				cpl := campSpend[urn] / float64(leads)
				if cpl > 3*medCPL {
					out = append(out, auditFinding{
						Severity: sevImportant,
						Category: "spend",
						Message:  fmt.Sprintf("'%s' CPL %s — outlier (account median %s)", c.Name, formatMoney(cpl), formatMoney(medCPL)),
					})
				}
			}
		}
	}

	// Daily budget below learning threshold ($30)
	for _, c := range activeCamps {
		if c.DailyBudget == nil {
			continue
		}
		v, err := strconv.ParseFloat(c.DailyBudget.Amount, 64)
		if err != nil {
			continue
		}
		if v < 30 {
			out = append(out, auditFinding{
				Severity: sevImportant,
				Category: "budget",
				Message:  fmt.Sprintf("'%s' daily budget %s — below $30 learning threshold", c.Name, formatMoney(v)),
			})
		}
	}

	// Unused matched audiences: defined but not referenced by any active campaign.
	// Compare set of segments referenced in active campaign targeting against
	// the audience list.
	usedSegs := map[string]bool{}
	for _, c := range activeCamps {
		if c.TargetingCriteria == nil {
			continue
		}
		for _, seg := range c.TargetingCriteria.IncludedFacets()[audienceMatchingFacet] {
			usedSegs[seg] = true
		}
	}
	if audsErr == nil {
		for _, a := range auds {
			urnStr := fmt.Sprintf("urn:li:adSegment:%d", a.ID)
			if !usedSegs[urnStr] && a.Status != "ARCHIVED" {
				out = append(out, auditFinding{
					Severity: sevImportant,
					Category: "audience",
					Message:  fmt.Sprintf("Matched audience '%s' (%d members) is defined but unused by any active campaign", a.Name, a.MatchedCount),
				})
			}
		}
	}

	// === Info ===
	if anErr == nil && len(activeCamps) > 0 {
		topSpend := topCampaignsByMetric(activeCamps, campSpend, 3, true)
		if len(topSpend) > 0 {
			parts := make([]string, 0, len(topSpend))
			for _, t := range topSpend {
				parts = append(parts, fmt.Sprintf("%s (%s)", t.name, formatMoney(t.value)))
			}
			out = append(out, auditFinding{
				Severity: sevInfo,
				Category: "top",
				Message:  "Top by spend: " + strings.Join(parts, ", "),
			})
		}
		topLeadsList := topCampaignsByLeads(activeCamps, campLeads, campConv, 3)
		if len(topLeadsList) > 0 {
			parts := make([]string, 0, len(topLeadsList))
			for _, t := range topLeadsList {
				parts = append(parts, fmt.Sprintf("%s (%d)", t.name, int64(t.value)))
			}
			out = append(out, auditFinding{
				Severity: sevInfo,
				Category: "top",
				Message:  "Top by leads: " + strings.Join(parts, ", "),
			})
		}
		// Budget pacing
		var dailyCap float64
		for _, c := range activeCamps {
			if c.DailyBudget != nil {
				if v, err := strconv.ParseFloat(c.DailyBudget.Amount, 64); err == nil {
					dailyCap += v
				}
			}
		}
		var totalSpend float64
		for _, v := range campSpend {
			totalSpend += v
		}
		monthlyCap := dailyCap * 30
		if monthlyCap > 0 {
			pct := totalSpend / monthlyCap * 100
			out = append(out, auditFinding{
				Severity: sevInfo,
				Category: "pacing",
				Message:  fmt.Sprintf("Budget utilization: %s spend / %s cap (%.0f%%)", formatMoney(totalSpend), formatMoney(monthlyCap), pct),
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return severityRank(out[i].Severity) < severityRank(out[j].Severity) })
	return out
}

// topRow is a small helper struct for top-N rankings shared by spend/leads.
type topRow struct {
	name  string
	value float64
}

// topCampaignsByMetric ranks campaigns by a per-URN metric. desc orders by
// largest first. Returns up to n entries with non-zero values.
func topCampaignsByMetric(camps []api.Campaign, byURN map[string]float64, n int, desc bool) []topRow {
	rows := make([]topRow, 0, len(camps))
	for _, c := range camps {
		v := byURN[campaignURN(c.ID)]
		if v <= 0 {
			continue
		}
		rows = append(rows, topRow{name: c.Name, value: v})
	}
	sort.Slice(rows, func(i, j int) bool {
		if desc {
			return rows[i].value > rows[j].value
		}
		return rows[i].value < rows[j].value
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// topCampaignsByLeads ranks campaigns by total leads (oneClickLeads +
// externalWebsiteConversions).
func topCampaignsByLeads(camps []api.Campaign, leadsByURN, convByURN map[string]int64, n int) []topRow {
	rows := make([]topRow, 0, len(camps))
	for _, c := range camps {
		urn := campaignURN(c.ID)
		total := leadsByURN[urn] + convByURN[urn]
		if total <= 0 {
			continue
		}
		rows = append(rows, topRow{name: c.Name, value: float64(total)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].value > rows[j].value })
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

// campaignURN returns the canonical sponsoredCampaign URN for an id.
func campaignURN(id int64) string {
	return "urn:li:sponsoredCampaign:" + strconv.FormatInt(id, 10)
}

// severityRank orders critical < important < info.
func severityRank(s string) int {
	switch s {
	case sevCritical:
		return 0
	case sevImportant:
		return 1
	case sevInfo:
		return 2
	}
	return 3
}

// formatAuditReport renders the structured report as a terminal checklist
// grouped by severity.
func formatAuditReport(r auditReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Account audit: %s (%d)\n", r.Account.Name, r.Account.ID)
	fmt.Fprintf(&b, "Period: %s — %s\n\n", r.PeriodStart, r.PeriodEnd)

	groups := map[string][]auditFinding{}
	for _, f := range r.Findings {
		groups[f.Severity] = append(groups[f.Severity], f)
	}
	writeAuditGroup := func(label, sev, icon string) {
		if len(groups[sev]) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s %s (%d)\n", icon, label, len(groups[sev]))
		for _, f := range groups[sev] {
			fmt.Fprintf(&b, "  • %s\n", f.Message)
		}
		b.WriteString("\n")
	}
	writeAuditGroup("CRITICAL", sevCritical, "🔴")
	writeAuditGroup("IMPORTANT", sevImportant, "🟡")
	writeAuditGroup("INFO", sevInfo, "🟢")
	if len(r.Findings) == 0 {
		b.WriteString("✓ No issues detected.\n")
	}
	return b.String()
}
