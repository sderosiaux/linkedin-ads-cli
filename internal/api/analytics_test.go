package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestGetCampaignAnalytics_BuildsRawQuery(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		gotRaw = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"impressions":                  12345,
					"clicks":                       234,
					"costInUsd":                    "78.90",
					"externalWebsiteConversions":   5,
					"oneClickLeads":                2,
					"approximateUniqueImpressions": 9000,
				},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetCampaignAnalytics(context.Background(), c, "12345", start, end, "ALL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: %d", len(rows))
	}

	// Verify the unescaped tuple syntax landed verbatim.
	if !strings.Contains(gotRaw, "q=analytics") {
		t.Errorf("missing q=analytics: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "pivot=CAMPAIGN") {
		t.Errorf("missing pivot=CAMPAIGN: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "timeGranularity=ALL") {
		t.Errorf("missing timeGranularity: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "dateRange=(start:(year:2026,month:1,day:1),end:(year:2026,month:1,day:31))") {
		t.Errorf("dateRange not in raw form: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "accounts=List(urn%3Ali%3AsponsoredAccount%3A12345)") {
		t.Errorf("accounts list not in raw form: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "fields=") {
		t.Errorf("missing fields param: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "costInUsd") {
		t.Errorf("fields should include costInUsd: %s", gotRaw)
	}

	r0 := rows[0]
	if r0.Impressions != 12345 || r0.Clicks != 234 || r0.CostInUsd != "78.90" {
		t.Errorf("row[0]: %+v", r0)
	}
	if r0.Conversions != 5 || r0.OneClickLeads != 2 || r0.Reach != 9000 {
		t.Errorf("row[0] derived: %+v", r0)
	}
}

func TestAnalyticsRow_PivotValues(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"impressions": 10,
					"clicks":      1,
					"costInUsd":   "0.50",
					"pivotValues": []string{"urn:li:sponsoredCampaign:42"},
				},
			},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetCampaignAnalytics(context.Background(), c, "12345", start, end, "ALL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: %d", len(rows))
	}
	if len(rows[0].PivotValues) != 1 || rows[0].PivotValues[0] != "urn:li:sponsoredCampaign:42" {
		t.Errorf("PivotValues: %v", rows[0].PivotValues)
	}
}

func TestGetCreativeAnalytics(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[{"impressions":1,"clicks":2,"costInUsd":"3"}]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetCreativeAnalytics(context.Background(), c, "42", start, end, "ALL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: %d", len(rows))
	}
	if !strings.Contains(gotRaw, "pivot=CREATIVE") {
		t.Errorf("missing pivot=CREATIVE: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
		t.Errorf("missing campaigns list: %s", gotRaw)
	}
}

func TestGetDemographicsAnalytics(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if _, err := GetDemographicsAnalytics(context.Background(), c, "42", "JOB_FUNCTION", start, end); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotRaw, "pivot=JOB_FUNCTION") {
		t.Errorf("missing pivot=JOB_FUNCTION: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
		t.Errorf("missing campaigns list: %s", gotRaw)
	}
}

func TestGetDailyTrendsAnalytics_AccountScope(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if _, err := GetDailyTrendsAnalytics(context.Background(), c, "12345", "", start, end); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotRaw, "timeGranularity=DAILY") {
		t.Errorf("missing DAILY: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "accounts=List(urn%3Ali%3AsponsoredAccount%3A12345)") {
		t.Errorf("missing accounts: %s", gotRaw)
	}
}

func TestGetSingleCampaignAnalytics(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[{"impressions":100,"clicks":5,"costInUsd":"7.50"}]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetSingleCampaignAnalytics(context.Background(), c, "42", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Impressions != 100 {
		t.Errorf("rows: %+v", rows)
	}
	if !strings.Contains(gotRaw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
		t.Errorf("missing campaigns: %s", gotRaw)
	}
}

func TestAnalyticsRow_VideoMetrics(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"impressions":                   500,
					"clicks":                        10,
					"costInUsd":                     "2.50",
					"videoViews":                    100,
					"videoStarts":                   80,
					"videoCompletions":              30,
					"videoFirstQuartileCompletions": 60,
					"videoMidpointCompletions":      45,
					"videoThirdQuartileCompletions": 35,
				},
			},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetCreativeAnalytics(context.Background(), c, "42", start, end, "ALL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len: %d", len(rows))
	}
	r := rows[0]
	if r.VideoViews != 100 {
		t.Errorf("VideoViews: %d", r.VideoViews)
	}
	if r.VideoStarts != 80 {
		t.Errorf("VideoStarts: %d", r.VideoStarts)
	}
	if r.VideoCompletions != 30 {
		t.Errorf("VideoCompletions: %d", r.VideoCompletions)
	}
	if r.VideoQ1 != 60 {
		t.Errorf("VideoQ1: %d", r.VideoQ1)
	}
	if r.VideoMidpoint != 45 {
		t.Errorf("VideoMidpoint: %d", r.VideoMidpoint)
	}
	if r.VideoQ3 != 35 {
		t.Errorf("VideoQ3: %d", r.VideoQ3)
	}
}

func TestGetSingleCampaignGroupAnalytics(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[{"impressions":500,"clicks":25,"costInUsd":"15.00"}]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetSingleCampaignGroupAnalytics(context.Background(), c, "99", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Impressions != 500 {
		t.Errorf("rows: %+v", rows)
	}
	if !strings.Contains(gotRaw, "pivot=CAMPAIGN_GROUP") {
		t.Errorf("missing pivot: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "campaignGroups=List(urn%3Ali%3AsponsoredCampaignGroup%3A99)") {
		t.Errorf("missing campaignGroups: %s", gotRaw)
	}
}

func TestGetDailyTrendsAnalytics_CampaignScope(t *testing.T) {
	t.Parallel()
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	if _, err := GetDailyTrendsAnalytics(context.Background(), c, "", "42", start, end); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotRaw, "timeGranularity=DAILY") {
		t.Errorf("missing DAILY: %s", gotRaw)
	}
	if !strings.Contains(gotRaw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
		t.Errorf("missing campaigns: %s", gotRaw)
	}
}
