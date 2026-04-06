package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
