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
	accountURN := urn.Wrap(urn.Account, accountID)
	raw := fmt.Sprintf("q=analytics&pivot=CAMPAIGN&timeGranularity=%s&dateRange=%s&accounts=List(%s)",
		granularity, formatDateRange(start, end), accountURN)
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
