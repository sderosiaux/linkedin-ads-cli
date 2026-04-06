package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_ShowsHelp(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(out.String(), "linkedin-ads") {
		t.Fatalf("expected name in help, got: %s", out.String())
	}
}

func TestRootCommand_RejectsUnknownArgs(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"bogus"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown arg, got nil")
	}
}

func TestRootCommand_HelpIncludesDescription(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Inspect and manage") {
		t.Fatalf("expected Long description in help, got: %s", out.String())
	}
}
