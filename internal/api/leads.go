package api

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// LeadForm is a LinkedIn lead generation form decoded from /leadGenForms.
type LeadForm struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Account        string  `json:"account"`
	Locale         *Locale `json:"locale,omitempty"`
	Headline       string  `json:"headline,omitempty"`
	Description    string  `json:"description,omitempty"`
	CreatedAt      int64   `json:"createdAt,omitempty"`
	LastModifiedAt int64   `json:"lastModifiedAt,omitempty"`
}

// ListLeadForms returns lead-gen forms under the given account id (bare).
func ListLeadForms(ctx context.Context, c *client.Client, accountID string, limit int) ([]LeadForm, error) {
	q := url.Values{}
	q.Set("q", "account")
	q.Set("account", urn.Wrap(urn.Account, accountID))
	var out []LeadForm
	if err := client.PaginateStartCount(ctx, c, "/leadGenForms", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// LeadPerformanceRow is a single /adAnalytics row pivoted by LEAD_GEN_FORM.
// pivotValue holds the lead-gen form URN.
type LeadPerformanceRow struct {
	Form             string `json:"pivotValue,omitempty"`
	Impressions      int64  `json:"impressions"`
	Clicks           int64  `json:"clicks"`
	LeadGenFormOpens int64  `json:"leadGenFormOpens,omitempty"`
	LeadSubmissions  int64  `json:"oneClickLeads,omitempty"`
	CostInUsd        string `json:"costInUsd"`
}

// GetLeadPerformance returns per-form performance rows for an account over the
// given date range. When formID is non-empty, results are filtered to that
// single form; otherwise rows are returned for every form active in the range.
//
// pivot=LEAD_GEN_FORM is not verified against production — adjust if LinkedIn
// rejects it. The CLI command surfaces the raw error to the user in that case.
func GetLeadPerformance(ctx context.Context, c *client.Client, accountID, formID string, start, end time.Time) ([]LeadPerformanceRow, error) {
	accountURN := urn.Wrap(urn.Account, accountID)
	parts := []string{
		"q=analytics",
		"pivot=LEAD_GEN_FORM",
		"timeGranularity=ALL",
		"dateRange=" + formatDateRange(start, end),
		"accounts=List(" + accountURN + ")",
	}
	if formID != "" {
		parts = append(parts, "leadGenForms=List(urn:li:leadGenForm:"+formID+")")
	}
	rawQuery := strings.Join(parts, "&")
	var page struct {
		Elements []LeadPerformanceRow `json:"elements"`
	}
	if err := c.GetJSONRawQuery(ctx, "/adAnalytics", rawQuery, &page); err != nil {
		return nil, err
	}
	return page.Elements, nil
}
