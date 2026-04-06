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
	if !strings.Contains(gotRaw, "accounts=List(urn:li:sponsoredAccount:12345)") {
		t.Errorf("accounts list not in raw form: %s", gotRaw)
	}

	r0 := rows[0]
	if r0.Impressions != 12345 || r0.Clicks != 234 || r0.CostInUsd != "78.90" {
		t.Errorf("row[0]: %+v", r0)
	}
	if r0.Conversions != 5 || r0.OneClickLeads != 2 || r0.Reach != 9000 {
		t.Errorf("row[0] derived: %+v", r0)
	}
}
