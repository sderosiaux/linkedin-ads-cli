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

func TestAudiencesList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dmpSegments" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("account") != "urn:li:sponsoredAccount:777" {
			t.Errorf("account: %q", r.URL.Query().Get("account"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 1, "name": "Aud", "type": "LOOKALIKE", "status": "READY"},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "audiences", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 1`) {
		t.Errorf("got: %s", out.String())
	}
}

func TestAudiencesInUse_AggregatesAcrossCampaigns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAccounts/777/adCampaigns" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":            10,
					"name":          "Architect NAMER",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
					"targetingCriteria": map[string]any{
						"include": map[string]any{
							"and": []map[string]any{
								{"or": map[string]any{
									"urn:li:adTargetingFacet:audienceMatchingSegments": []string{
										"urn:li:adSegment:62755117",
										"urn:li:adSegment:65010452",
									},
								}},
							},
						},
					},
				},
				{
					"id":            11,
					"name":          "Architect EMEA",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
					"targetingCriteria": map[string]any{
						"include": map[string]any{
							"and": []map[string]any{
								{"or": map[string]any{
									"urn:li:adTargetingFacet:audienceMatchingSegments": []string{
										"urn:li:adSegment:65010452",
									},
								}},
							},
						},
					},
				},
				{
					// PAUSED — should be filtered out by default.
					"id":            12,
					"name":          "Old",
					"status":        "PAUSED",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
					"targetingCriteria": map[string]any{
						"include": map[string]any{
							"and": []map[string]any{
								{"or": map[string]any{
									"urn:li:adTargetingFacet:audienceMatchingSegments": []string{
										"urn:li:adSegment:99999999",
									},
								}},
							},
						},
					},
				},
				{
					// No targeting — must be skipped silently.
					"id":            13,
					"name":          "Untargeted",
					"status":        "ACTIVE",
					"account":       "urn:li:sponsoredAccount:777",
					"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
					"type":          "SPONSORED_UPDATES",
					"objectiveType": "WEBSITE_VISIT",
					"costType":      "CPC",
				},
			},
			"metadata": map[string]any{},
		})
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "audiences", "in-use"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d: %s", len(got), out.String())
	}
	// Segment :65010452 is used by 2 campaigns — must come first.
	if got[0]["segment"] != "urn:li:adSegment:65010452" || got[0]["campaignCount"].(float64) != 2 {
		t.Errorf("expected :65010452 with count 2 first, got: %+v", got[0])
	}
	if got[1]["segment"] != "urn:li:adSegment:62755117" || got[1]["campaignCount"].(float64) != 1 {
		t.Errorf("expected :62755117 with count 1 second, got: %+v", got[1])
	}
	// PAUSED segment :99999999 must be absent (default status filter).
	for _, row := range got {
		if row["segment"] == "urn:li:adSegment:99999999" {
			t.Errorf("PAUSED-only segment should be filtered: %+v", row)
		}
	}
}

func TestAudiencesInUse_WithSpend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/adAccounts/777/adCampaigns":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"elements": []map[string]any{
					{
						"id": 10, "name": "C1", "status": "ACTIVE",
						"account":       "urn:li:sponsoredAccount:777",
						"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
						"type":          "SPONSORED_UPDATES",
						"objectiveType": "WEBSITE_VISIT", "costType": "CPC",
						"targetingCriteria": map[string]any{
							"include": map[string]any{"and": []map[string]any{
								{"or": map[string]any{"urn:li:adTargetingFacet:audienceMatchingSegments": []string{"urn:li:adSegment:1"}}},
							}},
						},
					},
					{
						"id": 11, "name": "C2", "status": "ACTIVE",
						"account":       "urn:li:sponsoredAccount:777",
						"campaignGroup": "urn:li:sponsoredCampaignGroup:111",
						"type":          "SPONSORED_UPDATES",
						"objectiveType": "WEBSITE_VISIT", "costType": "CPC",
						"targetingCriteria": map[string]any{
							"include": map[string]any{"and": []map[string]any{
								{"or": map[string]any{"urn:li:adTargetingFacet:audienceMatchingSegments": []string{"urn:li:adSegment:1", "urn:li:adSegment:2"}}},
							}},
						},
					},
				},
				"metadata": map[string]any{},
			})
		case "/adAnalytics":
			_, _ = w.Write([]byte(`{"elements":[
				{"pivotValues":["urn:li:sponsoredCampaign:10"],"impressions":1000,"clicks":100,"costInUsd":"100"},
				{"pivotValues":["urn:li:sponsoredCampaign:11"],"impressions":1000,"clicks":100,"costInUsd":"300"}
			]}`))
		default:
			t.Errorf("unexpected: %s", r.URL.Path)
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "audiences", "in-use", "--with-spend"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	var env struct {
		AccountSpend float64 `json:"accountSpend"`
		Segments     []struct {
			Segment          string  `json:"segment"`
			CampaignCount    int     `json:"campaignCount"`
			Spend            float64 `json:"spend"`
			PercentOfAccount float64 `json:"percentOfAccount"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v\n%s", err, out.String())
	}
	if env.AccountSpend != 400 {
		t.Errorf("accountSpend: %v", env.AccountSpend)
	}
	// Segment :1 is referenced by both C1 ($100) and C2 ($300) → $400.
	// Segment :2 only by C2 → $300.
	var s1, s2 float64
	for _, s := range env.Segments {
		if s.Segment == "urn:li:adSegment:1" {
			s1 = s.Spend
		}
		if s.Segment == "urn:li:adSegment:2" {
			s2 = s.Spend
		}
	}
	if s1 != 400 {
		t.Errorf("seg1 spend: %v want 400", s1)
	}
	if s2 != 300 {
		t.Errorf("seg2 spend: %v want 300", s2)
	}
}

func TestAudiencesList_EmptyState_Terminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[],"paging":{"start":0,"count":0,"total":0}}`))
	}))
	defer srv.Close()

	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "audiences", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "No matched or lookalike audiences for account 777") {
		t.Errorf("expected empty-state hint, got: %s", s)
	}
}
