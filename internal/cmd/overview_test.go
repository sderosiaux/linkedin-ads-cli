package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

// overviewServer returns a single httptest.Server that dispatches the four
// endpoints the overview command hits. analyticsHandler is optional; when nil
// the analytics endpoint replies with two rows summing to 183.45 / 15000 / 75.
func overviewServer(t *testing.T, analyticsHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/adAccounts/12345678/adCampaignGroups":
			_, _ = w.Write([]byte(`{"elements":[
				{"id":1,"name":"G1","status":"ACTIVE","account":"urn:li:sponsoredAccount:12345678"},
				{"id":2,"name":"G2","status":"DRAFT","account":"urn:li:sponsoredAccount:12345678"}
			],"metadata":{}}`))
		case r.URL.Path == "/adAccounts/12345678/adCampaigns" || r.URL.Path == "/adCampaigns":
			_, _ = w.Write([]byte(`{"elements":[
				{"id":1,"name":"C1","status":"ACTIVE","account":"urn:li:sponsoredAccount:12345678","campaignGroup":"urn:li:sponsoredCampaignGroup:1","type":"SPONSORED_UPDATES","objectiveType":"BRAND_AWARENESS","costType":"CPM"},
				{"id":2,"name":"C2","status":"PAUSED","account":"urn:li:sponsoredAccount:12345678","campaignGroup":"urn:li:sponsoredCampaignGroup:1","type":"SPONSORED_UPDATES","objectiveType":"BRAND_AWARENESS","costType":"CPM"}
			],"paging":{"start":0,"count":2,"total":2}}`))
		case strings.HasPrefix(r.URL.Path, "/adAccounts/"):
			_, _ = w.Write([]byte(`{"id":12345678,"name":"Acme","status":"ACTIVE","type":"BUSINESS","currency":"USD"}`))
		case r.URL.Path == "/adAnalytics":
			if analyticsHandler != nil {
				analyticsHandler(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"elements":[
				{"impressions":10000,"clicks":50,"costInUsd":"123.45"},
				{"impressions":5000,"clicks":25,"costInUsd":"60.00"}
			],"paging":{"start":0,"count":2,"total":2}}`))
		default:
			t.Logf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func writeOverviewConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "12345678"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	return cfgPath
}

func TestOverview_JSON(t *testing.T) {
	srv := overviewServer(t, nil)
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	cfgPath := writeOverviewConfig(t)

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "overview"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, out.String())
	}

	groups, _ := got["campaign_groups"].(map[string]any)
	if groups["active"].(float64) != 1 || groups["total"].(float64) != 2 {
		t.Errorf("campaign_groups: %v", groups)
	}
	camps, _ := got["campaigns"].(map[string]any)
	if camps["active"].(float64) != 1 || camps["paused"].(float64) != 1 || camps["total"].(float64) != 2 {
		t.Errorf("campaigns: %v", camps)
	}
	if got["spend_last_7d"].(float64) != 183.45 {
		t.Errorf("spend: %v", got["spend_last_7d"])
	}
	if got["impressions_last_7d"].(float64) != 15000 {
		t.Errorf("impressions: %v", got["impressions_last_7d"])
	}
	if got["clicks_last_7d"].(float64) != 75 {
		t.Errorf("clicks: %v", got["clicks_last_7d"])
	}
	acct, _ := got["account"].(map[string]any)
	if acct["name"].(string) != "Acme" || acct["currency"].(string) != "USD" {
		t.Errorf("account: %v", acct)
	}
}

func TestOverview_Terminal(t *testing.T) {
	srv := overviewServer(t, nil)
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	cfgPath := writeOverviewConfig(t)

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "overview"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "Acme") {
		t.Errorf("expected 'Acme' in terminal output: %s", s)
	}
	if !strings.Contains(s, "$183.45") {
		t.Errorf("expected '$183.45' in terminal output: %s", s)
	}
	if !strings.Contains(s, "1 active") {
		t.Errorf("expected '1 active' in terminal output: %s", s)
	}
}

func TestOverview_AnalyticsFailureDegrades(t *testing.T) {
	srv := overviewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	cfgPath := writeOverviewConfig(t)

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "overview"})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected analytics failure to degrade gracefully, got error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Acme") {
		t.Errorf("expected account meta still present: %s", s)
	}
	if !strings.Contains(s, "(unavailable)") {
		t.Errorf("expected '(unavailable)' marker, got: %s", s)
	}
}
