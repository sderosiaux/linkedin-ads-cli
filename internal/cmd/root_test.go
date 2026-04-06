package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_ShowsHelp(t *testing.T) {
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
