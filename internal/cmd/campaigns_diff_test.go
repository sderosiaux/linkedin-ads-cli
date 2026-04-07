package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestDiffStringSets_Basic(t *testing.T) {
	t.Parallel()
	a := []string{"x", "y", "z"}
	b := []string{"y", "z", "w"}
	onlyA, onlyB, shared := diffStringSets(uniqueSorted(a), uniqueSorted(b))
	if got := strings.Join(onlyA, ","); got != "x" {
		t.Errorf("onlyA: %q", got)
	}
	if got := strings.Join(onlyB, ","); got != "w" {
		t.Errorf("onlyB: %q", got)
	}
	if got := strings.Join(shared, ","); got != "y,z" {
		t.Errorf("shared: %q", got)
	}
}

func TestBuildCampaignDiff_TopLevelAndTargeting(t *testing.T) {
	t.Parallel()
	a := &api.Campaign{
		ID: 1, Name: "A", Status: "ACTIVE", Objective: "WEBSITE_VISIT", CostType: "CPM",
		DailyBudget: &api.Money{Amount: "100", CurrencyCode: "USD"},
		TargetingCriteria: &api.TargetingCriteria{
			Include: &api.TargetingInclude{And: []api.TargetingClause{
				{Or: map[string][]string{
					"urn:li:adTargetingFacet:titles":           {"urn:li:title:1", "urn:li:title:2", "urn:li:title:3"},
					"urn:li:adTargetingFacet:profileLocations": {"urn:li:geo:101"},
				}},
			}},
		},
	}
	b := &api.Campaign{
		ID: 2, Name: "B", Status: "PAUSED", Objective: "WEBSITE_VISIT", CostType: "CPC",
		DailyBudget: &api.Money{Amount: "100", CurrencyCode: "USD"},
		TargetingCriteria: &api.TargetingCriteria{
			Include: &api.TargetingInclude{And: []api.TargetingClause{
				{Or: map[string][]string{
					"urn:li:adTargetingFacet:titles":           {"urn:li:title:2", "urn:li:title:4"},
					"urn:li:adTargetingFacet:profileLocations": {"urn:li:geo:101"},
				}},
			}},
		},
	}
	env := buildCampaignDiff(a, b)
	if _, ok := env.TopLevel["status"]; !ok {
		t.Errorf("expected status diff: %+v", env.TopLevel)
	}
	if _, ok := env.TopLevel["costType"]; !ok {
		t.Errorf("expected costType diff: %+v", env.TopLevel)
	}
	if _, ok := env.TopLevel["dailyBudget"]; ok {
		t.Errorf("dailyBudget should NOT diff (same money)")
	}
	if _, ok := env.TopLevel["objectiveType"]; ok {
		t.Errorf("objectiveType should NOT diff")
	}
	var titles facetDiff
	for _, d := range env.Targeting.Include {
		if d.Facet == "urn:li:adTargetingFacet:titles" {
			titles = d
		}
	}
	if len(titles.OnlyA) != 2 || len(titles.OnlyB) != 1 || len(titles.Shared) != 1 {
		t.Errorf("titles diff wrong: %+v", titles)
	}
}

func TestCampaignsDiff_TerminalAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/adAccounts/777/adCampaigns/10":
			_, _ = w.Write([]byte(`{
				"id":10,"name":"Architect","status":"ACTIVE",
				"account":"urn:li:sponsoredAccount:777",
				"campaignGroup":"urn:li:sponsoredCampaignGroup:111",
				"type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPM",
				"targetingCriteria":{
					"include":{"and":[{"or":{"urn:li:adTargetingFacet:titles":["urn:li:title:1","urn:li:title:2"]}}]}
				}
			}`))
		case "/adAccounts/777/adCampaigns/20":
			_, _ = w.Write([]byte(`{
				"id":20,"name":"Developer","status":"PAUSED",
				"account":"urn:li:sponsoredAccount:777",
				"campaignGroup":"urn:li:sponsoredCampaignGroup:111",
				"type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPM",
				"targetingCriteria":{
					"include":{"and":[{"or":{"urn:li:adTargetingFacet:titles":["urn:li:title:2","urn:li:title:3"]}}]}
				}
			}`))
		default:
			t.Errorf("unexpected: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}

	// Terminal
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "campaigns", "diff", "10", "20"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute terminal: %v\n%s", err, out.String())
	}
	s := out.String()
	for _, want := range []string{
		"━━━ Campaign diff ━━━",
		"A: Architect (10)",
		"B: Developer (20)",
		"status:",
		"titles:",
		"A only: 1",
		"B only: 1",
		"shared: 1",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in terminal output:\n%s", want, s)
		}
	}

	// JSON
	root = NewRootCmd()
	out = &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "campaigns", "diff", "10", "20"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute json: %v\n%s", err, out.String())
	}
	var got campaignDiffEnvelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if got.A.ID != 10 || got.B.ID != 20 {
		t.Errorf("wrong ids: %+v", got)
	}
	if _, ok := got.TopLevel["status"]; !ok {
		t.Errorf("expected status diff, got: %+v", got.TopLevel)
	}
}
