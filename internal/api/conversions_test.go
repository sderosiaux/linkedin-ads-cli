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

func TestListConversions(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "account" {
			t.Errorf("q: %q", q.Get("q"))
		}
		if q.Get("account") != "urn:li:sponsoredAccount:12345" {
			t.Errorf("account: %q", q.Get("account"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":                               1,
					"name":                             "Signup",
					"type":                             "LANDING",
					"enabled":                          true,
					"attributionType":                  "LAST_TOUCH_BY_CAMPAIGN",
					"postClickAttributionWindowSize":   30,
					"viewThroughAttributionWindowSize": 7,
					"value": map[string]any{
						"amount":       "10.00",
						"currencyCode": "USD",
					},
					"account": "urn:li:sponsoredAccount:12345",
				},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	convs, err := ListConversions(context.Background(), c, "12345", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(convs) != 1 {
		t.Fatalf("len: %d", len(convs))
	}
	cv := convs[0]
	if cv.ID != 1 || cv.Name != "Signup" || !cv.Enabled {
		t.Errorf("conv: %+v", cv)
	}
	if cv.PostClickAttrWindow != 30 || cv.ViewThroughAttrWindow != 7 {
		t.Errorf("attr windows: %+v", cv)
	}
	if cv.Value == nil || cv.Value.Amount != "10.00" {
		t.Errorf("value: %+v", cv.Value)
	}
}

func TestGetConversionPerformance(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "q=analytics") {
			t.Errorf("missing q=analytics in: %s", raw)
		}
		if !strings.Contains(raw, "pivot=CONVERSION") {
			t.Errorf("missing pivot=CONVERSION in: %s", raw)
		}
		if !strings.Contains(raw, "accounts=List(urn%3Ali%3AsponsoredAccount%3A12345)") {
			t.Errorf("missing accounts list in: %s", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"pivotValue":                 "urn:li:lmsConversion:1",
					"impressions":                1000,
					"clicks":                     50,
					"externalWebsiteConversions": 7,
					"costInUsd":                  "12.34",
				},
				{
					"pivotValue":                 "urn:li:lmsConversion:2",
					"impressions":                500,
					"clicks":                     20,
					"externalWebsiteConversions": 3,
					"costInUsd":                  "5.00",
				},
			},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	rows, err := GetConversionPerformance(context.Background(), c, "12345", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len: %d", len(rows))
	}
	if rows[0].Conversion != "urn:li:lmsConversion:1" || rows[0].Impressions != 1000 || rows[0].Conversions != 7 {
		t.Errorf("row[0]: %+v", rows[0])
	}
}
