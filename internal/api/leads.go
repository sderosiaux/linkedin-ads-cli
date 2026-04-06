package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// LeadForm is a LinkedIn lead generation form decoded from /leadForms.
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

// leadFormsPage is the response envelope for lead forms listing.
type leadFormsPage struct {
	Elements json.RawMessage `json:"elements"`
	Paging   struct {
		Start int `json:"start"`
		Count int `json:"count"`
		Total int `json:"total"`
	} `json:"paging"`
}

// ListLeadForms returns lead-gen forms under the given account id (bare).
// Uses /leadForms with the compound owner param per LinkedIn API v2.
func ListLeadForms(ctx context.Context, c *client.Client, accountID string, limit int) ([]LeadForm, error) {
	// The owner param uses compound Rest.li syntax with raw parentheses.
	// Internal URN colons must be percent-encoded but parens stay literal.
	rawQuery := fmt.Sprintf("q=owner&owner=(sponsoredAccount:urn%%3Ali%%3AsponsoredAccount%%3A%s)", accountID)

	var accumulated []json.RawMessage
	start := 0
	pageSize := 500
	for {
		fullRaw := fmt.Sprintf("%s&start=%d&count=%d", rawQuery, start, pageSize)
		var page leadFormsPage
		if err := c.GetJSONRawQuery(ctx, "/leadForms", fullRaw, &page); err != nil {
			return nil, err
		}
		var raws []json.RawMessage
		if len(page.Elements) > 0 {
			if err := json.Unmarshal(page.Elements, &raws); err != nil {
				return nil, err
			}
		}
		accumulated = append(accumulated, raws...)
		if limit > 0 && len(accumulated) >= limit {
			accumulated = accumulated[:limit]
			break
		}
		if len(raws) < pageSize {
			break
		}
		start += pageSize
	}

	b, err := json.Marshal(accumulated)
	if err != nil {
		return nil, err
	}
	var out []LeadForm
	return out, json.Unmarshal(b, &out)
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
	accountURN := EncodeURNForList(urn.Wrap(urn.Account, accountID))
	parts := []string{
		"q=analytics",
		"pivot=LEAD_GEN_FORM",
		"timeGranularity=ALL",
		"dateRange=" + formatDateRange(start, end),
		"accounts=List(" + accountURN + ")",
	}
	if formID != "" {
		formURN := EncodeURNForList("urn:li:leadGenForm:" + formID)
		parts = append(parts, "leadGenForms=List("+formURN+")")
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
