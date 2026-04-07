package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/resolve"
	"github.com/spf13/cobra"
)

// audienceMatchingFacet is the LinkedIn targeting facet URN for matched
// audience segments. Centralised so the `in-use` aggregator stays in lockstep
// with the wire format.
const audienceMatchingFacet = "urn:li:adTargetingFacet:audienceMatchingSegments"

func newAudiencesCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "audiences",
		Short: "List matched and lookalike audiences",
	}
	root.AddCommand(newAudiencesListCmd(), newAudiencesInUseCmd())
	return root
}

// segmentUse is the aggregated per-segment row emitted by `audiences in-use`.
// It is public-shaped (exported JSON tags) so --json callers can consume it
// without extra plumbing.
type segmentUse struct {
	Segment          string           `json:"segment"`
	Name             string           `json:"name,omitempty"`
	CampaignCount    int              `json:"campaignCount"`
	Campaigns        []segmentCampRef `json:"campaigns"`
	Spend            float64          `json:"spend,omitempty"`
	PercentOfAccount float64          `json:"percentOfAccount,omitempty"`
}

// segmentUsageEnvelope is the JSON shape rendered when --with-spend is set.
// Adds the account-wide spend so consumers can verify the percentages.
type segmentUsageEnvelope struct {
	AccountSpend float64      `json:"accountSpend"`
	Segments     []segmentUse `json:"segments"`
}

type segmentCampRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func newAudiencesInUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "in-use",
		Short: "Show which matched audience segments are referenced by campaigns",
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
			if statusFilter == "" {
				statusFilter = "ACTIVE"
			}
			camps, err := api.ListCampaigns(cmd.Context(), c, accountID, "", 0)
			if err != nil {
				return err
			}
			usage := aggregateSegmentUsage(camps, statusFilter)

			if resolveFlag(cmd) {
				resolver := resolve.New(c, accountID)
				for i := range usage {
					if name := resolver.Resolve(cmd.Context(), usage[i].Segment); name != "" && name != usage[i].Segment {
						usage[i].Name = name
					}
				}
			}

			withSpend, _ := cmd.Flags().GetBool("with-spend")
			var accountSpend float64
			if withSpend {
				start, end, derr := parseDateRange(cmd)
				if derr != nil {
					return derr
				}
				rows, aerr := api.GetCampaignAnalytics(cmd.Context(), c, accountID, start, end, "ALL")
				if aerr != nil {
					return fmt.Errorf("fetch analytics for --with-spend: %w", aerr)
				}
				accountSpend = enrichSegmentUsageWithSpend(usage, rows)
			}

			if withSpend {
				envelope := segmentUsageEnvelope{
					AccountSpend: accountSpend,
					Segments:     usage,
				}
				return writeOutput(cmd, envelope, func() string {
					return formatSegmentUsageWithSpend(usage, accountSpend, statusFilter, accountID)
				})
			}
			return writeOutput(cmd, usage, func() string {
				if len(usage) == 0 {
					return fmt.Sprintf("No matched-audience segments referenced by %s campaigns in account %s.\n", statusFilter, accountID)
				}
				var b strings.Builder
				b.WriteString("SEGMENT                                  CAMPAIGNS  USED BY\n")
				for _, u := range usage {
					names := make([]string, 0, len(u.Campaigns))
					for _, cref := range u.Campaigns {
						names = append(names, cref.Name)
					}
					fmt.Fprintf(&b, "%-40s %9d  %s\n", truncate(u.Segment, 40), u.CampaignCount, strings.Join(names, ", "))
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("status", "ACTIVE", "Campaign status filter (default ACTIVE)")
	cmd.Flags().Bool("resolve", false, "Resolve segment URNs to names (usually 403 on most tokens)")
	cmd.Flags().Bool("with-spend", false, "Aggregate 30-day campaign spend per segment")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end) — used with --with-spend")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today) — used with --with-spend")
	return cmd
}

// enrichSegmentUsageWithSpend mutates usage in place, summing the spend of
// every campaign that references each segment, and returns the account-wide
// spend total. Multiple campaigns referencing the same segment have their
// spend added together; one campaign's spend is also counted under every
// segment it uses (the percentages don't sum to 100% — that's correct).
func enrichSegmentUsageWithSpend(usage []segmentUse, rows []api.AnalyticsRow) float64 {
	spendByCamp := map[int64]float64{}
	var account float64
	for _, r := range rows {
		v, _ := strconv.ParseFloat(r.CostInUsd, 64)
		account += v
		urn := pivotDisplay(r)
		const prefix = "urn:li:sponsoredCampaign:"
		if strings.HasPrefix(urn, prefix) {
			id, err := strconv.ParseInt(strings.TrimPrefix(urn, prefix), 10, 64)
			if err == nil {
				spendByCamp[id] += v
			}
		}
	}
	for i := range usage {
		var total float64
		for _, ref := range usage[i].Campaigns {
			total += spendByCamp[ref.ID]
		}
		usage[i].Spend = total
		if account > 0 {
			usage[i].PercentOfAccount = total / account * 100
		}
	}
	sort.SliceStable(usage, func(i, j int) bool { return usage[i].Spend > usage[j].Spend })
	return account
}

// formatSegmentUsageWithSpend renders the spend-enriched table.
func formatSegmentUsageWithSpend(usage []segmentUse, accountSpend float64, statusFilter, accountID string) string {
	if len(usage) == 0 {
		return fmt.Sprintf("No matched-audience segments referenced by %s campaigns in account %s.\n", statusFilter, accountID)
	}
	_ = time.Now // imports kept stable
	var b strings.Builder
	fmt.Fprintf(&b, "Account 30d spend: %s\n", formatMoney(accountSpend))
	b.WriteString("SEGMENT                                  CAMPAIGNS  SPEND       % ACCOUNT  USED BY\n")
	for _, u := range usage {
		names := make([]string, 0, len(u.Campaigns))
		for _, cref := range u.Campaigns {
			names = append(names, cref.Name)
		}
		fmt.Fprintf(&b, "%-40s %9d  %10s  %8s  %s\n",
			truncate(truncateURN(u.Segment, 4), 40),
			u.CampaignCount,
			formatMoney(u.Spend),
			fmt.Sprintf("%.0f%%", u.PercentOfAccount),
			strings.Join(names, ", "))
	}
	return b.String()
}

// aggregateSegmentUsage walks the campaign list, keeps only those with a
// matching status, and returns per-segment usage sorted by descending
// campaign count (then by segment URN for stable output). Campaigns without
// TargetingCriteria are silently skipped.
func aggregateSegmentUsage(camps []api.Campaign, status string) []segmentUse {
	bySeg := map[string][]segmentCampRef{}
	for _, camp := range camps {
		if status != "" && !strings.EqualFold(camp.Status, status) {
			continue
		}
		if camp.TargetingCriteria == nil {
			continue
		}
		segs := camp.TargetingCriteria.IncludedFacets()[audienceMatchingFacet]
		seen := map[string]bool{}
		for _, s := range segs {
			if seen[s] {
				continue
			}
			seen[s] = true
			bySeg[s] = append(bySeg[s], segmentCampRef{ID: camp.ID, Name: camp.Name})
		}
	}
	out := make([]segmentUse, 0, len(bySeg))
	for seg, refs := range bySeg {
		out = append(out, segmentUse{
			Segment:       seg,
			CampaignCount: len(refs),
			Campaigns:     refs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CampaignCount != out[j].CampaignCount {
			return out[i].CampaignCount > out[j].CampaignCount
		}
		return out[i].Segment < out[j].Segment
	})
	return out
}

func newAudiencesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List DMP segments under an account",
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
			auds, err := api.ListAudiences(cmd.Context(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, auds, func() string {
				if len(auds) == 0 {
					return fmt.Sprintf("No matched or lookalike audiences for account %s.\n", accountID)
				}
				var b strings.Builder
				b.WriteString("ID         NAME                TYPE         STATUS    AUDIENCE  MATCHED\n")
				for _, a := range auds {
					fmt.Fprintf(&b, "%-10d %-19s %-12s %-9s %8d %8d\n",
						a.ID, truncate(a.Name, 19), truncate(a.Type, 12), a.Status, a.AudienceCount, a.MatchedCount)
				}
				return b.String()
			})
		},
	}
	return cmd
}
