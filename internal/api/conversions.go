package api

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Conversion is a LinkedIn conversion definition decoded from /conversions.
type Conversion struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	Enabled               bool   `json:"enabled"`
	AttributionType       string `json:"attributionType"`
	PostClickAttrWindow   int64  `json:"postClickAttributionWindowSize,omitempty"`
	ViewThroughAttrWindow int64  `json:"viewThroughAttributionWindowSize,omitempty"`
	Value                 *Money `json:"value,omitempty"`
	Account               string `json:"account"`
}

// ListConversions returns conversion definitions under the given account id.
func ListConversions(ctx context.Context, c *client.Client, accountID string, limit int) ([]Conversion, error) {
	q := url.Values{}
	q.Set("q", "account")
	q.Set("account", urn.Wrap(urn.Account, accountID))
	var out []Conversion
	if err := client.PaginateStartCount(ctx, c, "/conversions", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// conversionPerformanceFields is the field set for conversion performance
// queries, matching the MCP's CONVERSION_METRICS.
const conversionPerformanceFields = "externalWebsiteConversions,externalWebsitePostClickConversions," +
	"costInUsd,conversionValueInLocalCurrency,externalWebsitePostViewConversions,pivotValues,dateRange"

// ConversionPerformanceRow is a single /adAnalytics row pivoted by CONVERSION.
// pivotValue holds the conversion URN; metrics are rolled up across the date
// range with timeGranularity=ALL.
type ConversionPerformanceRow struct {
	Conversion  string `json:"pivotValue,omitempty"`
	Impressions int64  `json:"impressions"`
	Clicks      int64  `json:"clicks"`
	Conversions int64  `json:"externalWebsiteConversions"`
	CostInUsd   string `json:"costInUsd"`
}

// ConversionEventInput is the request body for POST /conversionEvents. The
// LinkedIn Conversions API expects the conversion URN under the
// llaPartnerConversion namespace, an epoch-millis timestamp, a userIds array
// (CLI uses SHA256_EMAIL only), an optional value, and a caller-provided
// eventId for idempotency.
type ConversionEventInput struct {
	Conversion           string                `json:"conversion"`
	ConversionHappenedAt int64                 `json:"conversionHappenedAt"`
	User                 ConversionEventUser   `json:"user"`
	ConversionValue      *ConversionEventValue `json:"conversionValue,omitempty"`
	EventID              string                `json:"eventId,omitempty"`
}

// ConversionEventUser wraps the user matching identifiers — CLI only sends
// SHA256_EMAIL.
type ConversionEventUser struct {
	UserIDs []ConversionUserID `json:"userIds"`
}

// ConversionUserID is a single id-type / id-value pair.
type ConversionUserID struct {
	IDType  string `json:"idType"`
	IDValue string `json:"idValue"`
}

// ConversionEventValue carries the optional cash value (currency + amount).
type ConversionEventValue struct {
	CurrencyCode string `json:"currencyCode"`
	Amount       string `json:"amount"`
}

// PostConversionEvent uploads a single offline conversion event to LinkedIn's
// Conversions API. Returns the new event id from X-RestLi-Id when present.
//
// The token must have the "Conversions API" product enabled — without it,
// LinkedIn returns 403 with a hint to request the product in the developer
// portal.
func PostConversionEvent(ctx context.Context, c *client.Client, in *ConversionEventInput) (string, error) {
	return c.PostJSON(ctx, "/conversionEvents", in, nil)
}

// GetConversionPerformance returns per-conversion performance rows for the
// account over the given date range.
//
// pivot=CONVERSION is not verified against production — adjust if LinkedIn
// rejects it. The CLI command surfaces the raw error to the user in that case.
func GetConversionPerformance(ctx context.Context, c *client.Client, accountID string, start, end time.Time) ([]ConversionPerformanceRow, error) {
	accountURN := EncodeURNForList(urn.Wrap(urn.Account, accountID))
	rawQuery := fmt.Sprintf("q=analytics&pivot=CONVERSION&timeGranularity=ALL&dateRange=%s&accounts=List(%s)&fields=%s",
		formatDateRange(start, end), accountURN, conversionPerformanceFields)
	var page struct {
		Elements []ConversionPerformanceRow `json:"elements"`
	}
	if err := c.GetJSONRawQuery(ctx, "/adAnalytics", rawQuery, &page); err != nil {
		return nil, err
	}
	return page.Elements, nil
}
