package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newCreativesCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "creatives",
		Short: "List and inspect ad creatives",
	}
	root.AddCommand(newCreativesListCmd(), newCreativesGetCmd())
	return root
}

func newCreativesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List creatives under a campaign",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			campaignID, _ := cmd.Flags().GetString("campaign")
			if campaignID == "" {
				return errors.New("--campaign required")
			}
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			creatives, err := api.ListCreatives(cmd.Context(), c, campaignID, limitFlag(cmd))
			if err != nil {
				return err
			}
			return writeOutput(cmd, creatives, func() string {
				if len(creatives) == 0 {
					return fmt.Sprintf("No creatives on campaign %s.\n", campaignID)
				}
				var b strings.Builder
				b.WriteString("ID                                 STATUS    REVIEW    CAMPAIGN\n")
				for _, cr := range creatives {
					fmt.Fprintf(&b, "%-34s %-9s %-9s %s\n",
						truncate(cr.ID, 34), cr.Status, cr.Review, cr.Campaign)
				}
				return b.String()
			}, compactCreative)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (required)")
	return cmd
}

func newCreativesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single creative by URN-style id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			cr, err := api.GetCreative(cmd.Context(), c, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, cr, func() string {
				return fmt.Sprintf("ID:       %s\nStatus:   %s\nIntended: %s\nReview:   %s\nCampaign: %s\n",
					cr.ID, cr.Status, cr.IntendedStatus, cr.Review, cr.Campaign)
			})
		},
	}
}
