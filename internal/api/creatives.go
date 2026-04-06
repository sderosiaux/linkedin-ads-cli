package api

import (
	"context"
	"fmt"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Creative is a LinkedIn ad creative decoded from /adCreatives. Note that the
// id is a URN string ("urn:li:sponsoredCreative:<n>") not an int64.
type Creative struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	IntendedStatus string `json:"intendedStatus"`
	Campaign       string `json:"campaign"`
	Review         string `json:"review,omitempty"`
	CreatedAt      int64  `json:"createdAt,omitempty"`
	LastModifiedAt int64  `json:"lastModifiedAt,omitempty"`
}

// ListCreatives returns creatives under the given campaign id (bare).
// If limit > 0, iteration stops after limit items.
func ListCreatives(ctx context.Context, c *client.Client, campaignID string, limit int) ([]Creative, error) {
	campURN := urn.Wrap(urn.Campaign, campaignID)
	rawQuery := fmt.Sprintf("q=criteria&campaigns=List(%s)", campURN)
	var out []Creative
	if err := client.PaginateStartCountRaw(ctx, c, "/adCreatives", rawQuery, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCreative fetches a single creative by URN-style id.
func GetCreative(ctx context.Context, c *client.Client, id string) (*Creative, error) {
	var cr Creative
	if err := c.GetJSON(ctx, "/adCreatives/"+id, nil, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}
