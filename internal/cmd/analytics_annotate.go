package cmd

import (
	"sort"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

// annotateFlag returns true when --annotate is set on cmd. Defaults to false.
func annotateFlag(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool("annotate")
	if err != nil {
		return false
	}
	return v
}

// addAnnotateFlag wires --annotate on a command.
func addAnnotateFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("annotate", false, "Highlight outlier rows (best/worst CPL/CTR, low CTR, high CPL)")
}

// flag tag constants used both by terminal renderer and JSON _flags field.
const (
	flagBestCPL  = "best_cpl"
	flagWorstCPL = "worst_cpl"
	flagBestCTR  = "best_ctr"
	flagWorstCTR = "worst_ctr"
	flagLowCTR   = "low_ctr"
	flagHighCPL  = "high_cpl"
)

// annotationsFor walks the rows and returns one tag list per row containing
// the flag names that apply. The longest input list is returned so the caller
// can index by row index. Empty input returns nil.
func annotationsFor(rows []api.AnalyticsRow) [][]string {
	if len(rows) == 0 {
		return nil
	}
	out := make([][]string, len(rows))

	type stat struct {
		idx int
		ctr float64
		cpl float64
	}
	stats := make([]stat, 0, len(rows))
	ctrs := make([]float64, 0, len(rows))
	cpls := make([]float64, 0, len(rows))
	for i, r := range rows {
		d := r.DerivedMetrics()
		s := stat{idx: i, ctr: d["ctr"], cpl: d["cpl"]}
		stats = append(stats, s)
		if r.Impressions > 0 {
			ctrs = append(ctrs, d["ctr"])
		}
		if r.OneClickLeads+r.Conversions > 0 {
			cpls = append(cpls, d["cpl"])
		}
	}

	medianCTR := median(ctrs)
	medianCPL := median(cpls)

	// Best/worst CTR (highest is best)
	bestCTR, worstCTR := -1, -1
	bestCTRVal := -1.0
	worstCTRVal := -1.0
	for _, s := range stats {
		if rows[s.idx].Impressions == 0 {
			continue
		}
		if bestCTR == -1 || s.ctr > bestCTRVal {
			bestCTR = s.idx
			bestCTRVal = s.ctr
		}
		if worstCTR == -1 || s.ctr < worstCTRVal {
			worstCTR = s.idx
			worstCTRVal = s.ctr
		}
	}
	// Best/worst CPL (lowest cost-per-lead is best)
	bestCPL, worstCPL := -1, -1
	bestCPLVal := -1.0
	worstCPLVal := -1.0
	for _, s := range stats {
		if rows[s.idx].OneClickLeads+rows[s.idx].Conversions == 0 {
			continue
		}
		if bestCPL == -1 || s.cpl < bestCPLVal {
			bestCPL = s.idx
			bestCPLVal = s.cpl
		}
		if worstCPL == -1 || s.cpl > worstCPLVal {
			worstCPL = s.idx
			worstCPLVal = s.cpl
		}
	}

	for i, s := range stats {
		var tags []string
		if i == bestCPL && len(rows) > 1 {
			tags = append(tags, flagBestCPL)
		}
		if i == worstCPL && len(rows) > 1 && bestCPL != worstCPL {
			tags = append(tags, flagWorstCPL)
		}
		if i == bestCTR && len(rows) > 1 {
			tags = append(tags, flagBestCTR)
		}
		if i == worstCTR && len(rows) > 1 && bestCTR != worstCTR {
			tags = append(tags, flagWorstCTR)
		}
		if rows[i].Impressions > 0 && medianCTR > 0 && s.ctr < medianCTR*0.7 {
			tags = append(tags, flagLowCTR)
		}
		if rows[i].OneClickLeads+rows[i].Conversions > 0 && medianCPL > 0 && s.cpl > medianCPL*2.5 {
			tags = append(tags, flagHighCPL)
		}
		out[i] = tags
	}
	return out
}

// median returns the median of a float64 slice (0 when empty).
func median(in []float64) float64 {
	if len(in) == 0 {
		return 0
	}
	cp := make([]float64, len(in))
	copy(cp, in)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

// formatRowFlags renders a tag list into a short emoji-decorated string.
func formatRowFlags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		switch t {
		case flagBestCPL:
			parts = append(parts, "🟢 best CPL")
		case flagWorstCPL:
			parts = append(parts, "🔴 worst CPL")
		case flagBestCTR:
			parts = append(parts, "🟢 best CTR")
		case flagWorstCTR:
			parts = append(parts, "🔴 worst CTR")
		case flagLowCTR:
			parts = append(parts, "⚠️ low CTR")
		case flagHighCPL:
			parts = append(parts, "⚠️ high CPL")
		}
	}
	return strings.Join(parts, ", ")
}
