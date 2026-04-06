package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// AnalyticsRow is a single row from /adAnalytics. Numeric metrics decode as
// int64; cost is left as a decimal string to match LinkedIn's representation.
type AnalyticsRow struct {
	DateRange           map[string]any `json:"dateRange,omitempty"`
	Pivot               string         `json:"pivot,omitempty"`
	PivotValue          string         `json:"pivotValue,omitempty"`
	Impressions         int64          `json:"impressions"`
	Clicks              int64          `json:"clicks"`
	CostInUsd           string         `json:"costInUsd"`
	CostInLocalCurrency string         `json:"costInLocalCurrency,omitempty"`
	Conversions         int64          `json:"externalWebsiteConversions,omitempty"`
	OneClickLeads       int64          `json:"oneClickLeads,omitempty"`
	Reach               int64          `json:"approximateUniqueImpressions,omitempty"`
	VideoViews          int64          `json:"videoViews,omitempty"`
	VideoStarts         int64          `json:"videoStarts,omitempty"`
	VideoCompletions    int64          `json:"videoCompletions,omitempty"`
	VideoQ1             int64          `json:"videoFirstQuartileCompletions,omitempty"`
	VideoMidpoint       int64          `json:"videoMidpointCompletions,omitempty"`
	VideoQ3             int64          `json:"videoThirdQuartileCompletions,omitempty"`
}

// formatDateRange renders a LinkedIn Rest.li dateRange tuple. The parens and
// colons MUST NOT be percent-escaped — pass through GetJSONRawQuery only.
func formatDateRange(start, end time.Time) string {
	return fmt.Sprintf("(start:(year:%d,month:%d,day:%d),end:(year:%d,month:%d,day:%d))",
		start.Year(), int(start.Month()), start.Day(),
		end.Year(), int(end.Month()), end.Day())
}

// GetCampaignAnalytics returns analytics rows pivoted by CAMPAIGN for the
// given account id and date range. granularity is one of DAILY, MONTHLY, ALL.
func GetCampaignAnalytics(ctx context.Context, c *client.Client, accountID string, start, end time.Time, granularity string) ([]AnalyticsRow, error) {
	if granularity == "" {
		granularity = "ALL"
	}
	accountURN := EncodeURNForList(urn.Wrap(urn.Account, accountID))
	raw := fmt.Sprintf("q=analytics&pivot=CAMPAIGN&timeGranularity=%s&dateRange=%s&accounts=List(%s)",
		granularity, formatDateRange(start, end), accountURN)
	return decodeAnalytics(ctx, c, raw)
}

// GetCreativeAnalytics returns rows pivoted by CREATIVE for a single campaign.
func GetCreativeAnalytics(ctx context.Context, c *client.Client, campaignID string, start, end time.Time, granularity string) ([]AnalyticsRow, error) {
	if granularity == "" {
		granularity = "ALL"
	}
	campURN := EncodeURNForList(urn.Wrap(urn.Campaign, campaignID))
	raw := fmt.Sprintf("q=analytics&pivot=CREATIVE&timeGranularity=%s&dateRange=%s&campaigns=List(%s)",
		granularity, formatDateRange(start, end), campURN)
	return decodeAnalytics(ctx, c, raw)
}

// GetDemographicsAnalytics returns rows pivoted by a demographic dimension
// (e.g. JOB_FUNCTION, INDUSTRY, SENIORITY, COMPANY_SIZE, COUNTRY, REGION) for
// a single campaign. Demographics roll up across the full date range so this
// always uses timeGranularity=ALL.
func GetDemographicsAnalytics(ctx context.Context, c *client.Client, campaignID, pivot string, start, end time.Time) ([]AnalyticsRow, error) {
	campURN := EncodeURNForList(urn.Wrap(urn.Campaign, campaignID))
	raw := fmt.Sprintf("q=analytics&pivot=%s&timeGranularity=ALL&dateRange=%s&campaigns=List(%s)",
		pivot, formatDateRange(start, end), campURN)
	return decodeAnalytics(ctx, c, raw)
}

// GetSingleCampaignAnalytics returns the rolled-up analytics row(s) for a
// single campaign. Used by `analytics compare`. timeGranularity=ALL.
func GetSingleCampaignAnalytics(ctx context.Context, c *client.Client, campaignID string, start, end time.Time) ([]AnalyticsRow, error) {
	campURN := EncodeURNForList(urn.Wrap(urn.Campaign, campaignID))
	raw := fmt.Sprintf("q=analytics&pivot=CAMPAIGN&timeGranularity=ALL&dateRange=%s&campaigns=List(%s)",
		formatDateRange(start, end), campURN)
	return decodeAnalytics(ctx, c, raw)
}

// GetDailyTrendsAnalytics returns rows with timeGranularity=DAILY scoped to
// either an account (when accountID is set) or a single campaign (when
// campaignID is set). Exactly one of the two should be non-empty; if both are
// set, campaignID wins.
func GetDailyTrendsAnalytics(ctx context.Context, c *client.Client, accountID, campaignID string, start, end time.Time) ([]AnalyticsRow, error) {
	scope := ""
	switch {
	case campaignID != "":
		scope = fmt.Sprintf("campaigns=List(%s)", EncodeURNForList(urn.Wrap(urn.Campaign, campaignID)))
	case accountID != "":
		scope = fmt.Sprintf("accounts=List(%s)", EncodeURNForList(urn.Wrap(urn.Account, accountID)))
	default:
		return nil, fmt.Errorf("daily trends needs an accountID or campaignID")
	}
	raw := fmt.Sprintf("q=analytics&pivot=CAMPAIGN&timeGranularity=DAILY&dateRange=%s&%s",
		formatDateRange(start, end), scope)
	return decodeAnalytics(ctx, c, raw)
}

// decodeAnalytics issues the raw-query GET against /adAnalytics and decodes the
// shared envelope into a slice of rows.
func decodeAnalytics(ctx context.Context, c *client.Client, rawQuery string) ([]AnalyticsRow, error) {
	var env struct {
		Elements json.RawMessage `json:"elements"`
	}
	if err := c.GetJSONRawQuery(ctx, "/adAnalytics", rawQuery, &env); err != nil {
		return nil, err
	}
	if len(env.Elements) == 0 {
		return nil, nil
	}
	var rows []AnalyticsRow
	if err := json.Unmarshal(env.Elements, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
