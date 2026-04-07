package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

// TestCampaignsGet_PacingShownEvenWithEmptyAnalytics is a regression: a paused
// campaign with a daily budget but zero analytics rows must still print
// "Last 30d spend: $0.00", because the next "Avg daily" line references that
// number. Omitting it leaves the reader staring at "Avg daily: $0.00 (0% of
// cap)" with no spend total to anchor the math.
func TestCampaignsGet_PacingShownEvenWithEmptyAnalytics(t *testing.T) {
	startMs := time.Now().AddDate(0, 0, -741).UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/adAccounts/777/adCampaigns/10":
			body := `{
				"id":10,"name":"X","status":"PAUSED",
				"account":"urn:li:sponsoredAccount:777",
				"campaignGroup":"urn:li:sponsoredCampaignGroup:111",
				"type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPM",
				"dailyBudget":{"amount":"40","currencyCode":"USD"},
				"runSchedule":{"start":` + strconv.FormatInt(startMs, 10) + `}
			}`
			_, _ = w.Write([]byte(body))
		case "/adAnalytics":
			_, _ = w.Write([]byte(`{"elements":[]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
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
	root.SetArgs([]string{"--config", cfgPath, "campaigns", "get", "10"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "Last 30d spend: $0.00") {
		t.Errorf("expected 'Last 30d spend: $0.00' line, got:\n%s", s)
	}
	if !strings.Contains(s, "Avg daily:") {
		t.Errorf("expected 'Avg daily:' line, got:\n%s", s)
	}
}
