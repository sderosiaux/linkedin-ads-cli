package cmd

import (
	"fmt"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
)

func newUseAccountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-account <id>",
		Short: "Set the default ad account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			c.DefaultAccount = args[0]
			if err := config.Save(path, c); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ Default account: %s\n", args[0])
			return err
		},
	}
}

func newCurrentAccountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current-account",
		Short: "Print the current default ad account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			if c.DefaultAccount == "" {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "(none)")
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), c.DefaultAccount)
			return err
		},
	}
}
