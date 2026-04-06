package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRoot is a minimal cobra root that exposes the same persistent flags
// the real CLI uses, so executeWrite can read --dry-run and --yes from it.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "linkedin-ads"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().Bool("yes", false, "")
	return root
}

func TestExecuteWriteDryRunPrintsPayloadAndSkipsFn(t *testing.T) {
	t.Parallel()
	root := newTestRoot()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	called := false
	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(c *cobra.Command, _ []string) error {
			return executeWrite(c, "POST /adCampaignGroups", map[string]any{"name": "X"}, func() error {
				called = true
				return nil
			})
		},
	}
	root.AddCommand(cmd)
	root.SetArgs([]string{"--dry-run", "noop"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Errorf("fn must NOT run in dry-run mode")
	}
	if !strings.Contains(out.String(), "POST /adCampaignGroups") {
		t.Errorf("dry-run output missing summary: %q", out.String())
	}
	if !strings.Contains(out.String(), `"name": "X"`) {
		t.Errorf("dry-run output missing payload: %q", out.String())
	}
}

func TestExecuteWriteYesRunsFn(t *testing.T) {
	t.Parallel()
	root := newTestRoot()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	called := false
	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(c *cobra.Command, _ []string) error {
			return executeWrite(c, "POST /adCampaignGroups", map[string]any{}, func() error {
				called = true
				return nil
			})
		},
	}
	root.AddCommand(cmd)
	root.SetArgs([]string{"--yes", "noop"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Errorf("fn must run when --yes is set")
	}
}

// uuidV4Pattern matches the canonical 8-4-4-4-12 hex layout, ignoring version
// nibble — the test only cares about shape.
var uuidV4Pattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func TestExecuteWrite_LogsCorrelationID(t *testing.T) {
	t.Parallel()
	root := newTestRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)

	called := false
	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(c *cobra.Command, _ []string) error {
			return executeWrite(c, "POST /adCampaignGroups", map[string]any{}, func() error {
				called = true
				return nil
			})
		},
	}
	root.AddCommand(cmd)
	root.SetArgs([]string{"--yes", "noop"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("fn must run when --yes is set")
	}
	if !strings.Contains(stderr.String(), "correlation-id: ") {
		t.Errorf("expected 'correlation-id: ' on stderr, got: %q", stderr.String())
	}
	if !uuidV4Pattern.MatchString(stderr.String()) {
		t.Errorf("expected UUID-shaped correlation id on stderr, got: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "correlation-id") {
		t.Errorf("correlation-id must not appear on stdout, got: %q", stdout.String())
	}
}

func TestExecuteWrite_DryRun_NoCorrelationID(t *testing.T) {
	t.Parallel()
	root := newTestRoot()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)

	called := false
	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(c *cobra.Command, _ []string) error {
			return executeWrite(c, "POST /adCampaignGroups", map[string]any{}, func() error {
				called = true
				return nil
			})
		},
	}
	root.AddCommand(cmd)
	root.SetArgs([]string{"--dry-run", "noop"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Errorf("fn must NOT run in dry-run mode")
	}
	if strings.Contains(stderr.String(), "correlation-id") || strings.Contains(stdout.String(), "correlation-id") {
		t.Errorf("dry-run should NOT emit correlation-id; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
