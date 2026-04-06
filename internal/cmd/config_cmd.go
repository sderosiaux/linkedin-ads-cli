package cmd

import (
	"fmt"
	"regexp"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
)

var yyyymm = regexp.MustCompile(`^\d{6}$`)

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Show and modify configuration",
	}
	c.AddCommand(newConfigShowCmd(), newConfigSetVersionCmd())
	return c
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the current config (token masked)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			token := "(none)"
			if c.Token != "" {
				token = "***"
			}
			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(out, "path:            %s\n", path); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "token:           %s\n", token); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(out, "default_account: %s\n", defaultOr(c.DefaultAccount, "(none)")); err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "api_version:     %s\n", defaultOr(c.APIVersion, "(none)"))
			return err
		},
	}
}

func newConfigSetVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-version <YYYYMM>",
		Short: "Set the LinkedIn-Version header value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yyyymm.MatchString(args[0]) {
				return fmt.Errorf("invalid version %q: expected YYYYMM (6 digits)", args[0])
			}
			path := configPathFrom(cmd)
			c, err := config.Load(path)
			if err != nil {
				return err
			}
			c.APIVersion = args[0]
			if err := config.Save(path, c); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "✓ api_version: %s\n", args[0])
			return err
		},
	}
}

func defaultOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
