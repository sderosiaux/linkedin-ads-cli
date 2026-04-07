package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newLeadsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "leads",
		Short: "Inspect lead-gen forms and submissions",
	}
	root.AddCommand(newLeadsFormsCmd(), newLeadsPerformanceCmd())
	return root
}

func newLeadsFormsCmd() *cobra.Command {
	forms := &cobra.Command{
		Use:   "forms",
		Short: "Manage lead-gen forms",
	}
	forms.AddCommand(newLeadsFormsListCmd())
	return forms
}

func newLeadsPerformanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Lead-gen form performance breakdown over a date range (default: last 30 days)",
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
			rows, err := api.GetLeadPerformance(cmd.Context(), c, accountID, "", start, end)
			if err != nil {
				return err
			}
			if lim := limitFlag(cmd); lim > 0 && len(rows) > lim {
				rows = rows[:lim]
			}
			derived := derivedFlag(cmd)
			return writeOutput(cmd, rows, func() string {
				var b strings.Builder
				if derived {
					b.WriteString("FORM                                    IMPRESSIONS   CLICKS   OPENS   SUBMITS  SPEND       CTR     CPM\n")
				} else {
					b.WriteString("FORM                                    IMPRESSIONS   CLICKS   OPENS   SUBMITS  COST\n")
				}
				for _, r := range rows {
					name := truncate(truncateURN(r.Form, 4), 40)
					if derived {
						ctr := 0.0
						cpm := 0.0
						if r.Impressions > 0 {
							ctr = float64(r.Clicks) / float64(r.Impressions)
							if v, err := strconv.ParseFloat(r.CostInUsd, 64); err == nil {
								cpm = v / float64(r.Impressions) * 1000
							}
						}
						fmt.Fprintf(&b, "%-40s %11s %8s %7d %8d %10s %7s %8s\n",
							name, formatInt(r.Impressions), formatInt(r.Clicks), r.LeadGenFormOpens, r.LeadSubmissions,
							formatMoneyString(r.CostInUsd), formatPercent(ctr), formatMoney(cpm))
					} else {
						fmt.Fprintf(&b, "%-40s %11s %8s %7d %8d %s\n",
							name, formatInt(r.Impressions), formatInt(r.Clicks), r.LeadGenFormOpens, r.LeadSubmissions, formatMoneyString(r.CostInUsd))
					}
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	cmd.Flags().Bool("derived", true, "Show CTR/CPM columns (default: on in terminal, off in --json)")
	return cmd
}

func newLeadsFormsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List lead-gen forms under an account",
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
			forms, err := api.ListLeadForms(cmd.Context(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, forms, func() string {
				if len(forms) == 0 {
					return fmt.Sprintf("No lead-gen forms for account %s.\n", accountID)
				}
				var b strings.Builder
				b.WriteString("ID         NAME                STATE     VERSION\n")
				for _, f := range forms {
					fmt.Fprintf(&b, "%-10d %-19s %-9s %d\n",
						f.ID, truncate(f.Name, 19), f.State, f.VersionID)
				}
				return b.String()
			})
		},
	}
	return cmd
}
