package cmd

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

// facetDiff is the per-facet diff between two campaigns' targeting maps.
type facetDiff struct {
	Facet  string   `json:"facet"`
	OnlyA  []string `json:"only_a,omitempty"`
	OnlyB  []string `json:"only_b,omitempty"`
	Shared []string `json:"shared,omitempty"`
}

// targetingDiff captures include/exclude diffs.
type targetingDiff struct {
	Include []facetDiff `json:"include"`
	Exclude []facetDiff `json:"exclude"`
}

// topLevelChange is one differing top-level field.
type topLevelChange struct {
	A any `json:"a"`
	B any `json:"b"`
}

// campaignDiffEnvelope is the structured shape rendered when --json is set.
type campaignDiffEnvelope struct {
	A         campaignRef               `json:"a"`
	B         campaignRef               `json:"b"`
	TopLevel  map[string]topLevelChange `json:"topLevel"`
	Targeting targetingDiff             `json:"targeting"`
}

type campaignRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func newCampaignsDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <id-a> <id-b>",
		Short: "Diff two campaigns: top-level fields + targeting facets",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			var (
				ca, cb     *api.Campaign
				errA, errB error
				wg         sync.WaitGroup
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				ca, errA = api.GetCampaign(ctx, c, accountID, args[0])
			}()
			go func() {
				defer wg.Done()
				cb, errB = api.GetCampaign(ctx, c, accountID, args[1])
			}()
			wg.Wait()
			if errA != nil {
				return fmt.Errorf("get %s: %w", args[0], errA)
			}
			if errB != nil {
				return fmt.Errorf("get %s: %w", args[1], errB)
			}

			env := buildCampaignDiff(ca, cb)
			return writeOutput(cmd, env, func() string { return formatCampaignDiff(env) })
		},
	}
	return cmd
}

// buildCampaignDiff produces the structured diff envelope shared by terminal
// and JSON renderers. It computes top-level field deltas and per-facet value
// deltas for both include and exclude targeting clauses.
func buildCampaignDiff(a, b *api.Campaign) campaignDiffEnvelope {
	env := campaignDiffEnvelope{
		A:        campaignRef{ID: a.ID, Name: a.Name},
		B:        campaignRef{ID: b.ID, Name: b.Name},
		TopLevel: map[string]topLevelChange{},
	}

	addIfDiff := func(name string, x, y any) {
		if !equalAny(x, y) {
			env.TopLevel[name] = topLevelChange{A: x, B: y}
		}
	}
	addIfDiff("status", a.Status, b.Status)
	addIfDiff("objectiveType", a.Objective, b.Objective)
	addIfDiff("costType", a.CostType, b.CostType)
	addIfDiff("dailyBudget", moneyJSON(a.DailyBudget), moneyJSON(b.DailyBudget))
	addIfDiff("unitCost", moneyJSON(a.UnitCost), moneyJSON(b.UnitCost))
	addIfDiff("locale", localeStr(a.Locale), localeStr(b.Locale))

	env.Targeting.Include = diffFacetMaps(facetMap(a.TargetingCriteria, true), facetMap(b.TargetingCriteria, true))
	env.Targeting.Exclude = diffFacetMaps(facetMap(a.TargetingCriteria, false), facetMap(b.TargetingCriteria, false))
	return env
}

// facetMap returns either the include or exclude facet map of a campaign,
// always non-nil so callers can safely range it.
func facetMap(t *api.TargetingCriteria, include bool) map[string][]string {
	if t == nil {
		return map[string][]string{}
	}
	if include {
		return t.IncludedFacets()
	}
	return t.ExcludedFacets()
}

// diffFacetMaps walks the union of facet keys and emits a facetDiff per facet
// containing only_a, only_b, shared. Facets present in both with identical
// values are still emitted (only the shared list is non-empty) so the renderer
// can show "shared: N values".
func diffFacetMaps(a, b map[string][]string) []facetDiff {
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	out := make([]facetDiff, 0, len(keys))
	for k := range keys {
		av := uniqueSorted(a[k])
		bv := uniqueSorted(b[k])
		onlyA, onlyB, shared := diffStringSets(av, bv)
		out = append(out, facetDiff{
			Facet:  k,
			OnlyA:  onlyA,
			OnlyB:  onlyB,
			Shared: shared,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Facet < out[j].Facet })
	return out
}

// diffStringSets returns (only-in-a, only-in-b, in-both) for two sorted slices.
func diffStringSets(a, b []string) (onlyA, onlyB, shared []string) {
	bset := map[string]struct{}{}
	for _, v := range b {
		bset[v] = struct{}{}
	}
	aset := map[string]struct{}{}
	for _, v := range a {
		aset[v] = struct{}{}
	}
	for _, v := range a {
		if _, ok := bset[v]; ok {
			shared = append(shared, v)
		} else {
			onlyA = append(onlyA, v)
		}
	}
	for _, v := range b {
		if _, ok := aset[v]; !ok {
			onlyB = append(onlyB, v)
		}
	}
	return onlyA, onlyB, shared
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// equalAny compares two values via fmt formatting for the simple field types
// the diff cares about. The Money/Locale path normalises through dedicated
// helpers so empty pointers and zero values compare cleanly.
func equalAny(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func moneyJSON(m *api.Money) string {
	if m == nil {
		return ""
	}
	return m.Amount + " " + m.CurrencyCode
}

func localeStr(l *api.Locale) string {
	if l == nil {
		return ""
	}
	return l.Language + "_" + l.Country
}

// formatCampaignDiff renders the human-readable diff block. Long value lists
// are abbreviated to the first 3 entries followed by "... (N more)".
func formatCampaignDiff(env campaignDiffEnvelope) string {
	var b strings.Builder
	b.WriteString("━━━ Campaign diff ━━━\n")
	fmt.Fprintf(&b, "A: %s (%d)\n", env.A.Name, env.A.ID)
	fmt.Fprintf(&b, "B: %s (%d)\n", env.B.Name, env.B.ID)
	b.WriteString("\nTOP-LEVEL:\n")
	if len(env.TopLevel) == 0 {
		b.WriteString("  (no differences)\n")
	} else {
		keys := make([]string, 0, len(env.TopLevel))
		for k := range env.TopLevel {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ch := env.TopLevel[k]
			fmt.Fprintf(&b, "  %s:\n    A: %v\n    B: %v\n", k, formatTopLevelValue(ch.A), formatTopLevelValue(ch.B))
		}
	}
	b.WriteString("\nTARGETING — INCLUDE:\n")
	writeFacetDiff(&b, env.Targeting.Include)
	b.WriteString("\nTARGETING — EXCLUDE:\n")
	writeFacetDiff(&b, env.Targeting.Exclude)
	return b.String()
}

func formatTopLevelValue(v any) string {
	if s, ok := v.(string); ok && s == "" {
		return "(unset)"
	}
	return fmt.Sprintf("%v", v)
}

// writeFacetDiff prints one facetDiff per facet, abbreviating long value
// lists. A facet whose only_a / only_b / shared lists are all empty (which
// shouldn't happen but we guard against) is skipped.
func writeFacetDiff(b *strings.Builder, diffs []facetDiff) {
	if len(diffs) == 0 {
		b.WriteString("  (no facets)\n")
		return
	}
	hadAny := false
	for _, d := range diffs {
		if len(d.OnlyA) == 0 && len(d.OnlyB) == 0 && len(d.Shared) == 0 {
			continue
		}
		hadAny = true
		fmt.Fprintf(b, "  %s:\n", shortFacetName(d.Facet))
		if len(d.OnlyA) > 0 {
			fmt.Fprintf(b, "    A only: %d %s (%s)\n", len(d.OnlyA), pluralValues(len(d.OnlyA)), abbreviateValues(d.OnlyA))
		}
		if len(d.OnlyB) > 0 {
			fmt.Fprintf(b, "    B only: %d %s (%s)\n", len(d.OnlyB), pluralValues(len(d.OnlyB)), abbreviateValues(d.OnlyB))
		}
		if len(d.Shared) > 0 {
			fmt.Fprintf(b, "    shared: %d %s (%s)\n", len(d.Shared), pluralValues(len(d.Shared)), abbreviateValues(d.Shared))
		}
	}
	if !hadAny {
		b.WriteString("  (no facets)\n")
	}
}

func pluralValues(n int) string {
	if n == 1 {
		return "value"
	}
	return "values"
}

// abbreviateValues renders a list as "v1, v2, v3 ... (N more)" when long, or
// the full comma-joined list when short (≤3).
func abbreviateValues(vals []string) string {
	if len(vals) <= 3 {
		return strings.Join(vals, ", ")
	}
	return fmt.Sprintf("%s ... (%d more)", strings.Join(vals[:3], ", "), len(vals)-3)
}
