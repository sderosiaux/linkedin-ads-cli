package cmd

import (
	"bytes"
	"strings"
	"testing"
)

type testItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestWriteOutput_JSON(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--json"}); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	items := []testItem{{1, "a"}, {2, "b"}}
	if err := writeOutput(root, items, func() string { return "terminal" }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 1`) {
		t.Errorf("no json: %s", out.String())
	}
	if strings.Contains(out.String(), "terminal") {
		t.Errorf("terminal format leaked into json: %s", out.String())
	}
}

func TestWriteOutput_Terminal(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	if err := writeOutput(root, []testItem{{1, "a"}}, func() string { return "TERMINAL" }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "TERMINAL") {
		t.Errorf("expected terminal format, got: %s", out.String())
	}
}

func TestWriteOutput_Limit(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--json", "--limit", "2"}); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	items := []testItem{{1, "a"}, {2, "b"}, {3, "c"}}
	if err := writeOutput(root, items, func() string { return "" }); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), `"id": 3`) {
		t.Errorf("limit not applied: %s", out.String())
	}
}

func TestWriteOutput_Compact(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--json", "--compact"}); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	compact := func(a any) any {
		ti := a.(testItem)
		return map[string]int{"id": ti.ID}
	}
	items := []testItem{{1, "a"}}
	if err := writeOutput(root, items, func() string { return "" }, compact); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "name") || strings.Contains(s, `"a"`) {
		t.Errorf("compact did not strip fields: %s", s)
	}
	if !strings.Contains(s, `"id": 1`) {
		t.Errorf("id missing: %s", s)
	}
}

func TestWriteOutput_CompactWithoutFnIsNoop(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	if err := root.ParseFlags([]string{"--json", "--compact"}); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)

	items := []testItem{{1, "a"}}
	if err := writeOutput(root, items, func() string { return "" }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "a"`) {
		t.Errorf("compact without fn should be a no-op, got: %s", out.String())
	}
}

func TestLimitFlag_Default(t *testing.T) {
	t.Parallel()
	root := NewRootCmd()
	if got := limitFlag(root); got != 0 {
		t.Errorf("default limit should be 0, got %d", got)
	}
}
