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

func TestTruncateURN(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"urn:li:sponsoredCampaign:420247104", "...sponsoredCampaign:4202…"},
		{"urn:li:sponsoredCampaign:42", "...sponsoredCampaign:42"},
		{"urn:li:sponsoredCampaignGroup:674217704", "...sponsoredCampaignGroup:6742…"},
		{"plain string", "plain string"},
		{"urn:li:title:1", "...title:1"},
	}
	for _, c := range cases {
		got := truncateURN(c.in, 4)
		if got != c.want {
			t.Errorf("truncateURN(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestFormatMoneyString(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"1406.4072831443331", "$1,406.41"},
		{"22.500000000000001", "$22.50"},
		{"0", "$0.00"},
		{"", ""},
		{"abc", "abc"},
		{"1000000.5", "$1,000,000.50"},
	}
	for _, c := range cases {
		got := formatMoneyString(c.in)
		if got != c.want {
			t.Errorf("formatMoneyString(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   float64
		want string
	}{
		{0.0046, "0.46%"},
		{0.0, "0.00%"},
		{1.0, "100.00%"},
		{0.59, "59.00%"},
	}
	for _, c := range cases {
		got := formatPercent(c.in)
		if got != c.want {
			t.Errorf("formatPercent(%v): got %q want %q", c.in, got, c.want)
		}
	}
}
