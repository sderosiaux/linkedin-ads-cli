package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/confirm"
	"github.com/spf13/cobra"
)

// newCorrelationID returns a fresh RFC 4122 v4 UUID. Used as a CLI-side
// breadcrumb so users can grep their LinkedIn activity log against the
// stderr output of a write command.
func newCorrelationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}

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
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "correlation-id: %s\n", newCorrelationID()); err != nil {
		return err
	}
	return fn()
}
