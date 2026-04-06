package api

import (
	"context"
	"net/url"

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
