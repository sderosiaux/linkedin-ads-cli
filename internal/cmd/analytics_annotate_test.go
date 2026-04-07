package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestAnnotationsFor_BestWorst(t *testing.T) {
	t.Parallel()
	rows := []api.AnalyticsRow{
		// best CPL: 100/10 = 10, ctr 1%
		{Impressions: 10000, Clicks: 100, CostInUsd: "100", OneClickLeads: 10},
		// medium CPL: 200/5 = 40
		{Impressions: 10000, Clicks: 100, CostInUsd: "200", OneClickLeads: 5},
		// worst CPL: 1000/1 = 1000 — also high CPL outlier
		{Impressions: 10000, Clicks: 100, CostInUsd: "1000", OneClickLeads: 1},
	}
	flags := annotationsFor(rows)
	if len(flags) != 3 {
		t.Fatalf("expected 3 flag lists, got %d", len(flags))
	}
	if !contains(flags[0], flagBestCPL) {
		t.Errorf("row 0 expected best_cpl, got %v", flags[0])
	}
	if !contains(flags[2], flagWorstCPL) {
		t.Errorf("row 2 expected worst_cpl, got %v", flags[2])
	}
	if !contains(flags[2], flagHighCPL) {
		t.Errorf("row 2 expected high_cpl, got %v", flags[2])
	}
}

func TestAnnotationsFor_LowCTR(t *testing.T) {
	t.Parallel()
	rows := []api.AnalyticsRow{
		{Impressions: 10000, Clicks: 100, CostInUsd: "100", OneClickLeads: 1}, // ctr 1%
		{Impressions: 10000, Clicks: 100, CostInUsd: "100", OneClickLeads: 1}, // ctr 1%
		{Impressions: 10000, Clicks: 5, CostInUsd: "100", OneClickLeads: 1},   // ctr 0.05%
	}
	flags := annotationsFor(rows)
	if !contains(flags[2], flagLowCTR) {
		t.Errorf("row 2 expected low_ctr, got %v", flags[2])
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestAnalyticsCampaigns_AnnotateTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[
			{"impressions":10000,"clicks":100,"costInUsd":"100","oneClickLeads":10,"pivotValues":["urn:li:sponsoredCampaign:1"]},
			{"impressions":10000,"clicks":100,"costInUsd":"1000","oneClickLeads":1,"pivotValues":["urn:li:sponsoredCampaign:2"]}
		]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "analytics", "campaigns", "--annotate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "FLAG") {
		t.Errorf("expected FLAG header, got: %s", s)
	}
	if !strings.Contains(s, "best CPL") {
		t.Errorf("expected best CPL tag, got: %s", s)
	}
	if !strings.Contains(s, "worst CPL") {
		t.Errorf("expected worst CPL tag, got: %s", s)
	}
}

func TestAnalyticsCampaigns_AnnotateJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[
			{"impressions":10000,"clicks":100,"costInUsd":"100","oneClickLeads":10,"pivotValues":["urn:li:sponsoredCampaign:1"]},
			{"impressions":10000,"clicks":100,"costInUsd":"1000","oneClickLeads":1,"pivotValues":["urn:li:sponsoredCampaign:2"]}
		]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "analytics", "campaigns", "--annotate"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"_flags"`) {
		t.Errorf("expected _flags in JSON: %s", out.String())
	}
}
