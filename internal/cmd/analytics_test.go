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

func TestAnalyticsCampaigns_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CAMPAIGN") {
			t.Errorf("missing pivot: %s", raw)
		}
		if !strings.Contains(raw, "dateRange=(start:(year:2026,month:1,day:1),end:(year:2026,month:1,day:31))") {
			t.Errorf("dateRange shape wrong: %s", raw)
		}
		if !strings.Contains(raw, "accounts=List(urn%3Ali%3AsponsoredAccount%3A777)") {
			t.Errorf("accounts list shape wrong: %s", raw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"impressions": 1000, "clicks": 50, "costInUsd": "12.34"},
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
	root.SetArgs([]string{
		"--config", cfgPath, "--json",
		"analytics", "campaigns",
		"--start", "2026-01-01", "--end", "2026-01-31",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"impressions": 1000`) {
		t.Errorf("expected impressions, got: %s", out.String())
	}
}

func TestAnalyticsCampaigns_Compact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"dateRange":                  map[string]any{"start": map[string]any{"year": 2026, "month": 1, "day": 1}},
					"pivot":                      "CAMPAIGN",
					"pivotValue":                 "urn:li:sponsoredCampaign:42",
					"impressions":                1000,
					"clicks":                     50,
					"costInUsd":                  "12.34",
					"externalWebsiteConversions": 3,
					"oneClickLeads":              7,
				},
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
	root.SetArgs([]string{
		"--config", cfgPath, "--json", "--compact",
		"analytics", "campaigns",
		"--start", "2026-01-01", "--end", "2026-01-31",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{`"impressions"`, `"clicks"`, `"costInUsd"`, `"dateRange"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in compact whitelist, got: %s", want, s)
		}
	}
	for _, stripped := range []string{`"pivotValue"`, `"oneClickLeads"`, `"pivot"`} {
		if strings.Contains(s, stripped) {
			t.Errorf("%s should be stripped in compact, got: %s", stripped, s)
		}
	}
}

func TestAnalyticsCreatives_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CREATIVE") {
			t.Errorf("missing CREATIVE: %s", raw)
		}
		if !strings.Contains(raw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
			t.Errorf("missing campaign: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":11,"clicks":2,"costInUsd":"0.50"}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "analytics", "creatives", "--campaign", "42"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"impressions": 11`) {
		t.Errorf("got: %s", out.String())
	}
}

func TestAnalyticsDemographics_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=JOB_FUNCTION") {
			t.Errorf("missing pivot: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":7,"clicks":1,"costInUsd":"0.10"}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--json",
		"analytics", "demographics",
		"--campaign", "42", "--pivot", "JOB_FUNCTION",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"impressions": 7`) {
		t.Errorf("got: %s", out.String())
	}
}

func TestAnalyticsDemographics_BadPivot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "demographics",
		"--campaign", "42", "--pivot", "FOO",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid --pivot") {
		t.Errorf("expected pivot hint, got: %v", err)
	}
}

func TestAnalyticsDemographics_MemberPivot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=MEMBER_SENIORITY") {
			t.Errorf("missing pivot: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":3,"clicks":1,"costInUsd":"0.05"}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--json",
		"analytics", "demographics",
		"--campaign", "42", "--pivot", "MEMBER_SENIORITY",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"impressions": 3`) {
		t.Errorf("got: %s", out.String())
	}
}

func TestAnalyticsReach_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CAMPAIGN") {
			t.Errorf("missing pivot: %s", raw)
		}
		if !strings.Contains(raw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A42)") {
			t.Errorf("missing campaign: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":1,"clicks":0,"costInUsd":"0","approximateUniqueImpressions":900}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "analytics", "reach", "--campaign", "42"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"approximateUniqueImpressions": 900`) {
		t.Errorf("got: %s", out.String())
	}
}

func TestAnalyticsReach_92DayLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "reach",
		"--campaign", "42",
		"--start", "2026-01-01", "--end", "2026-06-01",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for >92 day range")
	}
	if !strings.Contains(err.Error(), "92 days") {
		t.Errorf("expected 92-day hint, got: %v", err)
	}
}

func TestAnalyticsDailyTrends_AccountDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "timeGranularity=DAILY") {
			t.Errorf("missing DAILY: %s", raw)
		}
		if !strings.Contains(raw, "accounts=List(urn%3Ali%3AsponsoredAccount%3A777)") {
			t.Errorf("missing accounts: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":1,"clicks":0,"costInUsd":"0"}]}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "analytics", "daily-trends"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyticsCompare_TwoCampaigns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		switch {
		case strings.Contains(raw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A1)"):
			_, _ = w.Write([]byte(`{"elements":[{"impressions":100,"clicks":10,"costInUsd":"5"}]}`))
		case strings.Contains(raw, "campaigns=List(urn%3Ali%3AsponsoredCampaign%3A2)"):
			_, _ = w.Write([]byte(`{"elements":[{"impressions":200,"clicks":40,"costInUsd":"10"}]}`))
		default:
			t.Errorf("unexpected raw: %s", raw)
		}
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "compare",
		"--a", "1", "--b", "2",
		"--metric", "clicks",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "100") || !strings.Contains(s, "200") {
		t.Errorf("expected both rows, got: %s", s)
	}
	if !strings.Contains(s, "300%") && !strings.Contains(s, "+300") {
		// 10 -> 40 = +300%
		t.Errorf("expected delta calc, got: %s", s)
	}
}

func TestAnalyticsCompare_CustomDates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "dateRange=(start:(year:2026,month:2,day:1),end:(year:2026,month:2,day:28))") {
			t.Errorf("dateRange wrong: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":50,"clicks":5,"costInUsd":"2"}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "compare",
		"--a", "1", "--b", "2",
		"--start", "2026-02-01", "--end", "2026-02-28",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyticsCompare_Groups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CAMPAIGN_GROUP") {
			t.Errorf("missing CAMPAIGN_GROUP pivot: %s", raw)
		}
		if !strings.Contains(raw, "campaignGroups=List(") {
			t.Errorf("missing campaignGroups: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[{"impressions":100,"clicks":10,"costInUsd":"5"}]}`))
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "compare",
		"--group-a", "10", "--group-b", "20",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "10") || !strings.Contains(s, "20") {
		t.Errorf("expected group labels, got: %s", s)
	}
}

func TestAnalyticsCompare_MutualExclusion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "compare",
		"--a", "1", "--b", "2", "--group-a", "10", "--group-b", "20",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for mutual exclusion")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually exclusive hint, got: %v", err)
	}
}

func TestAnalyticsCampaigns_BadDate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath,
		"analytics", "campaigns",
		"--start", "2026/01/01",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("expected 'invalid date' hint, got: %v", err)
	}
}
