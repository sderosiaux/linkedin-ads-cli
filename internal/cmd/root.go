package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	return &cobra.Command{
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
}
