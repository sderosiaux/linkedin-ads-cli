package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
	"github.com/spf13/cobra"
)

func newCreativesCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "creatives",
		Short: "List and inspect ad creatives",
	}
	root.AddCommand(
		newCreativesListCmd(),
		newCreativesGetCmd(),
		newCreativesCreateCmd(),
		newCreativesCreateInlineCmd(),
		newCreativesUpdateStatusCmd(),
	)
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
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			creatives, err := api.ListCreatives(cmd.Context(), c, accountID, campaignID, limitFlag(cmd))
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
						truncate(cr.ID, 34), cr.Status, cr.ReviewStatus(), cr.Campaign)
				}
				return b.String()
			}, compactCreative)
		},
	}
	cmd.Flags().String("campaign", "", "Campaign id (required)")
	return cmd
}

func newCreativesCreateCmd() *cobra.Command {
	var (
		campaignID string
		contentRef string
		status     string
		name       string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new creative referencing an existing post/share",
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
			in := &api.CreateCreativeInput{
				Campaign:       urn.Wrap(urn.Campaign, campaignID),
				IntendedStatus: strings.ToUpper(status),
			}
			if contentRef != "" {
				in.Content = &api.CreativeContent{Reference: contentRef}
			}
			if name != "" {
				in.Name = name
			}
			summary := fmt.Sprintf("POST /adAccounts/%s/creatives", accountID)
			return executeWrite(cmd, summary, in, func() error {
				id, err := api.CreateCreative(cmd.Context(), c, accountID, in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created creative %s\n", id)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign", "", "Campaign id (required)")
	cmd.Flags().StringVar(&contentRef, "content-reference", "", "Post/share URN to reference")
	cmd.Flags().StringVar(&status, "status", "ACTIVE", "Intended status (ACTIVE, PAUSED)")
	cmd.Flags().StringVar(&name, "name", "", "Optional creative name")
	_ = cmd.MarkFlagRequired("campaign")
	return cmd
}

func newCreativesCreateInlineCmd() *cobra.Command {
	var (
		campaignID  string
		orgID       string
		text        string
		imageURN    string
		imageTitle  string
		landingPage string
		cta         string
		status      string
		name        string
	)
	cmd := &cobra.Command{
		Use:   "create-inline",
		Short: "Create a creative with inline post content (no existing post needed)",
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
			in := &api.CreateInlineCreativeInput{
				Campaign:       campaignID,
				AccountID:      accountID,
				OrgID:          orgID,
				IntendedStatus: strings.ToUpper(status),
				Name:           name,
				Commentary:     text,
				ImageURN:       imageURN,
				ImageTitle:     imageTitle,
				LandingPageURL: landingPage,
				CTALabel:       cta,
			}
			summary := fmt.Sprintf("POST /adAccounts/%s/creatives?action=createInline", accountID)
			body := api.BuildInlineCreativeBody(accountID, in)
			return executeWrite(cmd, summary, body, func() error {
				id, err := api.CreateInlineCreative(cmd.Context(), c, accountID, in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created inline creative %s\n", id)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&campaignID, "campaign", "", "Campaign id (required)")
	cmd.Flags().StringVar(&orgID, "org", "", "Organization id (required)")
	cmd.Flags().StringVar(&text, "text", "", "Post commentary text (required)")
	cmd.Flags().StringVar(&imageURN, "image", "", "Image URN (optional)")
	cmd.Flags().StringVar(&imageTitle, "image-title", "", "Image title (optional)")
	cmd.Flags().StringVar(&landingPage, "landing-page", "", "Landing page URL (optional)")
	cmd.Flags().StringVar(&cta, "cta", "", "Call-to-action label e.g. LEARN_MORE (optional)")
	cmd.Flags().StringVar(&status, "status", "ACTIVE", "Intended status")
	cmd.Flags().StringVar(&name, "name", "", "Optional creative name")
	_ = cmd.MarkFlagRequired("campaign")
	_ = cmd.MarkFlagRequired("org")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

var validCreativeStatuses = map[string]struct{}{
	"ACTIVE":   {},
	"PAUSED":   {},
	"ARCHIVED": {},
}

func newCreativesUpdateStatusCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "update-status <urn-or-id>",
		Short: "Change the intended status of a creative",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status = strings.ToUpper(status)
			if _, ok := validCreativeStatuses[status]; !ok {
				return fmt.Errorf("invalid --status %q (want ACTIVE, PAUSED, or ARCHIVED)", status)
			}
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			urnOrID := args[0]
			payload := map[string]any{
				"patch": map[string]any{
					"$set": map[string]any{"intendedStatus": status},
				},
			}
			summary := fmt.Sprintf("POST /adAccounts/%s/creatives/%s (update-status → %s)", accountID, urnOrID, status)
			return executeWrite(cmd, summary, payload, func() error {
				if err := api.UpdateCreativeStatus(cmd.Context(), c, accountID, urnOrID, status); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Creative %s status set to %s\n", urnOrID, status)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "ACTIVE, PAUSED, or ARCHIVED (required)")
	_ = cmd.MarkFlagRequired("status")
	return cmd
}

func newCreativesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a single creative by numeric id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			accountID, err := accountIDFromFlagOrConfig(cmd, cfg)
			if err != nil {
				return err
			}
			cr, err := api.GetCreative(cmd.Context(), c, accountID, args[0])
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
