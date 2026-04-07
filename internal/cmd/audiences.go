package cmd

import (
	"fmt"
	"sort"
	"strings"

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
	Segment       string           `json:"segment"`
	Name          string           `json:"name,omitempty"`
	CampaignCount int              `json:"campaignCount"`
	Campaigns     []segmentCampRef `json:"campaigns"`
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
	return cmd
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
