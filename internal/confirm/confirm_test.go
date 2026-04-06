package confirm

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptYesShortAccepts(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("y\n")
	out := &bytes.Buffer{}
	ok, err := Prompt(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected true for 'y'")
	}
	if !strings.Contains(out.String(), "Continue?") || !strings.Contains(out.String(), "[y/N]") {
		t.Errorf("prompt missing: %q", out.String())
	}
}

func TestPromptYesLongAccepts(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("yes\n")
	out := &bytes.Buffer{}
	ok, err := Prompt(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected true for 'yes'")
	}
}

func TestPromptYesUppercaseAccepts(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("Y\n")
	out := &bytes.Buffer{}
	ok, err := Prompt(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected true for 'Y'")
	}
}

func TestPromptNoRejects(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("n\n")
	out := &bytes.Buffer{}
	ok, err := Prompt(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("expected false for 'n'")
	}
}

func TestPromptEmptyRejects(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("\n")
	out := &bytes.Buffer{}
	ok, err := Prompt(in, out, "Continue?")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("expected false for empty input")
	}
}

func TestPromptEOFRejects(t *testing.T) {
	t.Parallel()
	in := bytes.NewBufferString("")
	out := &bytes.Buffer{}
	ok, _ := Prompt(in, out, "Continue?")
	if ok {
		t.Errorf("expected false on EOF")
	}
}
