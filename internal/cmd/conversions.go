package cmd

import (
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newConversionsCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "conversions",
		Short: "List conversion definitions",
	}
	root.AddCommand(newConversionsListCmd(), newConversionsPerformanceCmd())
	return root
}

func newConversionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversion definitions under an account",
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
			convs, err := api.ListConversions(cmd.Context(), c, accountID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, convs, func() string {
				if len(convs) == 0 {
					return fmt.Sprintf("No conversion definitions for account %s.\n", accountID)
				}
				var b strings.Builder
				b.WriteString("ID         NAME                TYPE         ENABLED  ATTRIBUTION\n")
				for _, c := range convs {
					fmt.Fprintf(&b, "%-10d %-19s %-12s %-8t %s\n",
						c.ID, truncate(c.Name, 19), truncate(c.Type, 12), c.Enabled, c.AttributionType)
				}
				return b.String()
			})
		},
	}
	return cmd
}

func newConversionsPerformanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Conversion performance breakdown over a date range (default: last 30 days)",
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
			rows, err := api.GetConversionPerformance(cmd.Context(), c, accountID, start, end)
			if err != nil {
				return err
			}
			return writeOutput(cmd, rows, func() string {
				var b strings.Builder
				b.WriteString("CONVERSION                              IMPRESSIONS   CLICKS   CONV    COST\n")
				for _, r := range rows {
					fmt.Fprintf(&b, "%-40s %11d %8d %7d %s\n",
						truncate(r.Conversion, 40), r.Impressions, r.Clicks, r.Conversions, r.CostInUsd)
				}
				return b.String()
			})
		},
	}
	cmd.Flags().String("start", "", "Start date YYYY-MM-DD (default: 30 days before --end)")
	cmd.Flags().String("end", "", "End date YYYY-MM-DD (default: today)")
	return cmd
}
