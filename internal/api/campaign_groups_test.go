package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListCampaignGroups(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaignGroups" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "search" {
			t.Errorf("q: %q", q.Get("q"))
		}
		search := q.Get("search")
		if !strings.Contains(search, "urn:li:sponsoredAccount:12345") {
			t.Errorf("search missing account urn: %q", search)
		}
		if !strings.HasPrefix(search, "(account:(values:List(") {
			t.Errorf("search shape: %q", search)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":      111,
					"name":    "Q1 Brand",
					"status":  "ACTIVE",
					"account": "urn:li:sponsoredAccount:12345",
					"totalBudget": map[string]any{
						"amount":       "5000.00",
						"currencyCode": "USD",
					},
					"runSchedule": map[string]any{
						"start": 1700000000000,
						"end":   1710000000000,
					},
					"servingStatuses": []string{"RUNNABLE"},
				},
				{
					"id":      222,
					"name":    "Q2 Brand",
					"status":  "DRAFT",
					"account": "urn:li:sponsoredAccount:12345",
				},
			},
			"paging": map[string]any{"start": 0, "count": 2, "total": 2},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	groups, err := ListCampaignGroups(context.Background(), c, "12345", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("len: %d", len(groups))
	}
	g := groups[0]
	if g.ID != 111 || g.Name != "Q1 Brand" || g.Status != "ACTIVE" {
		t.Errorf("group[0]: %+v", g)
	}
	if g.Account != "urn:li:sponsoredAccount:12345" {
		t.Errorf("account: %q", g.Account)
	}
	if g.TotalBudget == nil || g.TotalBudget.Amount != "5000.00" || g.TotalBudget.CurrencyCode != "USD" {
		t.Errorf("totalBudget: %+v", g.TotalBudget)
	}
	if g.RunSchedule == nil || g.RunSchedule.Start != 1700000000000 {
		t.Errorf("runSchedule: %+v", g.RunSchedule)
	}
	if len(g.ServingStatuses) != 1 || g.ServingStatuses[0] != "RUNNABLE" {
		t.Errorf("servingStatuses: %+v", g.ServingStatuses)
	}
}

func TestGetCampaignGroup(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaignGroups/111" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":111,"name":"Q1","status":"ACTIVE","account":"urn:li:sponsoredAccount:12345"}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	g, err := GetCampaignGroup(context.Background(), c, "111")
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != 111 || g.Name != "Q1" {
		t.Errorf("group: %+v", g)
	}
}
