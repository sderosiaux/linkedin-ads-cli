package cmd

import (
	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "linkedin-ads",
		Short:         "LinkedIn Marketing API CLI",
		Long:          "Inspect and manage LinkedIn Ads campaigns from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.PersistentFlags().String("config", config.DefaultPath(), "config file path")
	root.PersistentFlags().Bool("json", false, "Output as JSON (LLM/scripts)")
	root.PersistentFlags().Bool("compact", false, "Minimal JSON fields (requires --json)")
	root.PersistentFlags().Int("limit", 0, "Cap array results at N items")
	root.AddCommand(
		newAuthCmd(),
		newUseAccountCmd(),
		newCurrentAccountCmd(),
		newConfigCmd(),
		newAccountsCmd(),
		newCampaignGroupsCmd(),
		newCampaignsCmd(),
		newCreativesCmd(),
		newAnalyticsCmd(),
		newAudiencesCmd(),
		newConversionsCmd(),
		newLeadsCmd(),
	)
	return root
}

// configPathFrom returns the config file path resolved from the root's
// persistent --config flag, falling back to config.DefaultPath() if empty.
func configPathFrom(cmd *cobra.Command) string {
	p, _ := cmd.Root().PersistentFlags().GetString("config")
	if p == "" {
		return config.DefaultPath()
	}
	return p
}
