package cmd

import (
	"fmt"
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
			formID, _ := cmd.Flags().GetString("form")
			rows, err := api.GetLeadPerformance(cmd.Context(), c, accountID, formID, start, end)
			if err != nil {
				return err
			}
			return writeOutput(cmd, rows, func() string {
				var b strings.Builder
				b.WriteString("FORM                                    IMPRESSIONS   CLICKS   OPENS   SUBMITS  COST\n")
				for _, r := range rows {
					fmt.Fprintf(&b, "%-40s %11d %8d %7d %8d %s\n",
						truncate(r.Form, 40), r.Impressions, r.Clicks, r.LeadGenFormOpens, r.LeadSubmissions, r.CostInUsd)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	cmd.Flags().String("form", "", "Filter by lead-gen form id")
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
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
				var b strings.Builder
				b.WriteString("ID         NAME                STATUS    HEADLINE\n")
				for _, f := range forms {
					fmt.Fprintf(&b, "%-10d %-19s %-9s %s\n",
						f.ID, truncate(f.Name, 19), f.Status, truncate(f.Headline, 40))
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("account", "", "Ad account id (default: current-account)")
	return cmd
}
