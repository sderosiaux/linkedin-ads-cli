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

func TestCreateCampaignGroup(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("X-LinkedIn-Id", "555")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	in := &CreateCampaignGroupInput{
		Account: "urn:li:sponsoredAccount:12345",
		Name:    "Q2 Brand",
		TotalBudget: &Money{
			CurrencyCode: "USD",
			Amount:       "5000",
		},
		RunSchedule: &DateRange{Start: 1745000000000},
	}
	id, err := CreateCampaignGroup(context.Background(), c, in)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups" {
		t.Errorf("path: %q", gotPath)
	}
	if id != "555" {
		t.Errorf("id: %q", id)
	}
	if gotBody["account"] != "urn:li:sponsoredAccount:12345" {
		t.Errorf("body account: %+v", gotBody)
	}
	if gotBody["name"] != "Q2 Brand" {
		t.Errorf("body name: %+v", gotBody)
	}
	// Defaults Status to DRAFT when not provided.
	if gotBody["status"] != "DRAFT" {
		t.Errorf("body status: %+v", gotBody)
	}
	tb, ok := gotBody["totalBudget"].(map[string]any)
	if !ok || tb["amount"] != "5000" || tb["currencyCode"] != "USD" {
		t.Errorf("body totalBudget: %+v", gotBody["totalBudget"])
	}
	rs, ok := gotBody["runSchedule"].(map[string]any)
	if !ok {
		t.Errorf("body runSchedule missing: %+v", gotBody)
	} else if int64(rs["start"].(float64)) != 1745000000000 {
		t.Errorf("body runSchedule start: %+v", rs)
	}
}

func TestUpdateCampaignGroupOnlyStatus(t *testing.T) {
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
	if err := UpdateCampaignGroup(context.Background(), c, "111", &UpdateCampaignGroupInput{
		Status: &status,
	}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups/111" {
		t.Errorf("path: %q", gotPath)
	}
	if gotRestliMethod != "PARTIAL_UPDATE" {
		t.Errorf("X-RestLi-Method: %q", gotRestliMethod)
	}
	expected := `{"patch":{"$set":{"status":"ACTIVE"}}}`
	if strings.TrimSpace(string(gotBodyRaw)) != expected {
		t.Errorf("body:\n got: %s\nwant: %s", string(gotBodyRaw), expected)
	}
}

func TestDeleteCampaignGroup(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	if err := DeleteCampaignGroup(context.Background(), c, "111"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method: %q", gotMethod)
	}
	if gotPath != "/adCampaignGroups/111" {
		t.Errorf("path: %q", gotPath)
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
