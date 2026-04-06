package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListCampaigns_AccountOnly(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaigns" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "search" {
			t.Errorf("q: %q", q.Get("q"))
		}
		search := q.Get("search")
		if !strings.Contains(search, "urn:li:sponsoredAccount:12345") {
			t.Errorf("search missing account: %q", search)
		}
		if strings.Contains(search, "campaignGroup") {
			t.Errorf("search should NOT contain campaignGroup when no group passed: %q", search)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":            10,
					"name":          "Test Campaign",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:12345",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
					"locale":        map[string]any{"country": "US", "language": "en"},
					"dailyBudget":   map[string]any{"amount": "50.00", "currencyCode": "USD"},
					"totalBudget":   map[string]any{"amount": "500.00", "currencyCode": "USD"},
					"unitCost":      map[string]any{"amount": "2.50", "currencyCode": "USD"},
					"runSchedule":   map[string]any{"start": 1700000000000, "end": 1710000000000},
				},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	camps, err := ListCampaigns(context.Background(), c, "12345", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(camps) != 1 {
		t.Fatalf("len: %d", len(camps))
	}
	c0 := camps[0]
	if c0.ID != 10 || c0.Name != "Test Campaign" || c0.CostType != "CPC" {
		t.Errorf("camp: %+v", c0)
	}
	if c0.Locale == nil || c0.Locale.Country != "US" || c0.Locale.Language != "en" {
		t.Errorf("locale: %+v", c0.Locale)
	}
	if c0.DailyBudget == nil || c0.DailyBudget.Amount != "50.00" {
		t.Errorf("dailyBudget: %+v", c0.DailyBudget)
	}
	if c0.Objective != "WEBSITE_VISIT" {
		t.Errorf("objective: %q", c0.Objective)
	}
}

func TestListCampaigns_WithGroup(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		if !strings.Contains(search, "urn:li:sponsoredAccount:12345") {
			t.Errorf("search missing account: %q", search)
		}
		if !strings.Contains(search, "urn:li:sponsoredCampaignGroup:99") {
			t.Errorf("search missing group: %q", search)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{},
			"paging":   map[string]any{"start": 0, "count": 0, "total": 0},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if _, err := ListCampaigns(context.Background(), c, "12345", "99", 0); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCampaign(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("X-LinkedIn-Id", "777")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	in := &CreateCampaignInput{
		Account:       "urn:li:sponsoredAccount:12345",
		CampaignGroup: "urn:li:sponsoredCampaignGroup:678",
		Name:          "Spring 2026",
		Type:          "SPONSORED_UPDATES",
		ObjectiveType: "BRAND_AWARENESS",
		Locale:        &Locale{Country: "US", Language: "en"},
		DailyBudget:   &Money{CurrencyCode: "USD", Amount: "100"},
	}
	id, err := CreateCampaign(context.Background(), c, in)
	if err != nil {
		t.Fatal(err)
	}
	if id != "777" {
		t.Errorf("id: %q", id)
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaigns" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotBody["account"] != "urn:li:sponsoredAccount:12345" {
		t.Errorf("body account: %+v", gotBody)
	}
	if gotBody["campaignGroup"] != "urn:li:sponsoredCampaignGroup:678" {
		t.Errorf("body campaignGroup: %+v", gotBody)
	}
	if gotBody["status"] != "DRAFT" {
		t.Errorf("body status: %+v", gotBody)
	}
	if gotBody["costType"] != "CPM" {
		t.Errorf("body costType (default CPM): %+v", gotBody)
	}
	if gotBody["objectiveType"] != "BRAND_AWARENESS" {
		t.Errorf("body objectiveType: %+v", gotBody)
	}
}

func TestUpdateCampaignOnlyStatus(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath, gotRestliMethod string
	var gotBodyRaw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRestliMethod = r.Header.Get("X-RestLi-Method")
		gotBodyRaw, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	status := "ACTIVE"
	if err := UpdateCampaign(context.Background(), c, "10", &UpdateCampaignInput{Status: &status}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/adCampaigns/10" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotRestliMethod != "PARTIAL_UPDATE" {
		t.Errorf("X-RestLi-Method: %q", gotRestliMethod)
	}
	expected := `{"patch":{"$set":{"status":"ACTIVE"}}}`
	if strings.TrimSpace(string(gotBodyRaw)) != expected {
		t.Errorf("body:\n got: %s\nwant: %s", string(gotBodyRaw), expected)
	}
}

func TestDeleteCampaign(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := DeleteCampaign(context.Background(), c, "10"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/adCampaigns/10" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestGetCampaign(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adCampaigns/10" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"ACTIVE","account":"urn:li:sponsoredAccount:12345","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	camp, err := GetCampaign(context.Background(), c, "10")
	if err != nil {
		t.Fatal(err)
	}
	if camp.ID != 10 {
		t.Errorf("id: %d", camp.ID)
	}
}
