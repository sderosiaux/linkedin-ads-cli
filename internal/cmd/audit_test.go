package cmd

import (
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
)

func TestComputeAuditFindings_DisabledConversion(t *testing.T) {
	t.Parallel()
	convs := []api.Conversion{
		{ID: 1, Name: "AWS Reg", Enabled: false},
		{ID: 2, Name: "Signup", Enabled: true},
	}
	out := computeAuditFindings(nil, nil, convs, nil, nil, nil, nil)
	found := false
	for _, f := range out {
		if strings.Contains(f.Message, "AWS Reg") && f.Severity == sevCritical {
			found = true
		}
	}
	if !found {
		t.Errorf("expected critical finding for disabled conversion: %+v", out)
	}
}

func TestComputeAuditFindings_BrokenAttribution(t *testing.T) {
	t.Parallel()
	camps := []api.Campaign{
		{ID: 1, Name: "C1", Status: "ACTIVE"},
		{ID: 2, Name: "C2", Status: "ACTIVE"},
	}
	rows := []api.AnalyticsRow{
		{PivotValues: []string{"urn:li:sponsoredCampaign:1"}, Impressions: 1000, Clicks: 5, CostInUsd: "200"},
		{PivotValues: []string{"urn:li:sponsoredCampaign:2"}, Impressions: 1000, Clicks: 5, CostInUsd: "200"},
	}
	out := computeAuditFindings(camps, rows, nil, nil, nil, nil, nil)
	found := false
	for _, f := range out {
		if strings.Contains(f.Message, "attribution may be broken") && f.Severity == sevCritical {
			found = true
		}
	}
	if !found {
		t.Errorf("expected broken attribution critical finding: %+v", out)
	}
}

func TestComputeAuditFindings_LowDailyBudget(t *testing.T) {
	t.Parallel()
	camps := []api.Campaign{
		{ID: 1, Name: "Tiny", Status: "ACTIVE", DailyBudget: &api.Money{Amount: "15", CurrencyCode: "USD"}},
		{ID: 2, Name: "Big", Status: "ACTIVE", DailyBudget: &api.Money{Amount: "100", CurrencyCode: "USD"}},
	}
	out := computeAuditFindings(camps, nil, nil, nil, nil, nil, nil)
	hits := 0
	for _, f := range out {
		if f.Severity == sevImportant && strings.Contains(f.Message, "below $30") {
			hits++
		}
	}
	if hits != 1 {
		t.Errorf("expected 1 low-budget warning, got %d: %+v", hits, out)
	}
}

func TestComputeAuditFindings_UnusedAudience(t *testing.T) {
	t.Parallel()
	auds := []api.Audience{
		{ID: 9999, Name: "ICP Accounts", MatchedCount: 100, Status: "READY"},
	}
	camps := []api.Campaign{
		{
			ID: 1, Name: "C1", Status: "ACTIVE",
			TargetingCriteria: &api.TargetingCriteria{
				Include: &api.TargetingInclude{And: []api.TargetingClause{
					{Or: map[string][]string{
						"urn:li:adTargetingFacet:audienceMatchingSegments": {"urn:li:adSegment:8888"},
					}},
				}},
			},
		},
	}
	out := computeAuditFindings(camps, nil, nil, auds, nil, nil, nil)
	found := false
	for _, f := range out {
		if strings.Contains(f.Message, "ICP Accounts") && strings.Contains(f.Message, "unused") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unused-audience warning: %+v", out)
	}
}

func TestSeverityRank_Ordering(t *testing.T) {
	t.Parallel()
	if severityRank(sevCritical) >= severityRank(sevImportant) {
		t.Errorf("critical should rank before important")
	}
	if severityRank(sevImportant) >= severityRank(sevInfo) {
		t.Errorf("important should rank before info")
	}
}
