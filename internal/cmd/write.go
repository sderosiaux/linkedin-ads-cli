package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/confirm"
	"github.com/spf13/cobra"
)

// executeWrite handles dry-run, confirmation prompt, and execution for any
// write command.
//
// summary is the short description shown in the prompt (e.g. "POST
// /adCampaignGroups"). payload is rendered as pretty JSON in the dry-run
// preview and the confirmation preview. fn performs the actual API call.
//
// The dry-run preview is intentionally human-readable JSON describing the
// request shape, NOT the structured --json envelope used by read commands.
// --dry-run and --json are orthogonal.
func executeWrite(cmd *cobra.Command, summary string, payload any, fn func() error) error {
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if dryRunFlag(cmd) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), summary); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(pretty)); err != nil {
			return err
		}
		return nil
	}
	if !yesFlag(cmd) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), summary); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(pretty)); err != nil {
			return err
		}
		ok, err := confirm.Prompt(os.Stdin, cmd.OutOrStdout(), "Continue?")
		if err != nil {
			return err
		}
		if !ok {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Aborted."); err != nil {
				return err
			}
			return nil
		}
	}
	return fn()
}
