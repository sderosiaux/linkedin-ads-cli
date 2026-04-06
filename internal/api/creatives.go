package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Creative is a LinkedIn ad creative decoded from
// /adAccounts/{accountId}/creatives. Note that the id is a URN string
// ("urn:li:sponsoredCreative:<n>") not an int64.
type Creative struct {
	ID             string          `json:"id"`
	Status         string          `json:"status"`
	IntendedStatus string          `json:"intendedStatus"`
	Campaign       string          `json:"campaign"`
	Review         *CreativeReview `json:"review,omitempty"`
	CreatedAt      int64           `json:"createdAt,omitempty"`
	LastModifiedAt int64           `json:"lastModifiedAt,omitempty"`
}

// CreativeReview is the review status envelope returned by the creatives API.
type CreativeReview struct {
	Status string `json:"status"`
}

// ReviewStatus returns the review status string, or "" if review is nil.
func (c *Creative) ReviewStatus() string {
	if c.Review == nil {
		return ""
	}
	return c.Review.Status
}

// creativesPage is the response envelope for creatives listing.
type creativesPage struct {
	Elements json.RawMessage `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

// ListCreatives returns creatives under the given account. When campaignID is
// non-empty, results are filtered to that campaign via the campaigns=List(...)
// query param. Uses metadata.nextPageToken pagination.
// If limit > 0, iteration stops after limit items.
func ListCreatives(ctx context.Context, c *client.Client, accountID, campaignID string, limit int) ([]Creative, error) {
	path := "/adAccounts/" + accountID + "/creatives"

	// Build raw query: q=criteria plus optional campaign filter.
	// We must use raw query because List() parentheses and URN colons
	// cannot survive url.Values.Encode().
	rawBase := "q=criteria"
	if campaignID != "" {
		campURN := url.QueryEscape(urn.Wrap(urn.Campaign, campaignID))
		rawBase += fmt.Sprintf("&campaigns=List(%s)", campURN)
	}

	var accumulated []json.RawMessage
	token := ""
	for {
		rq := rawBase
		if token != "" {
			rq += "&pageToken=" + url.QueryEscape(token)
		}
		var page creativesPage
		if err := c.GetJSONRawQuery(ctx, path, rq, &page); err != nil {
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
		if page.Metadata.NextPageToken == "" {
			break
		}
		token = page.Metadata.NextPageToken
	}

	b, err := json.Marshal(accumulated)
	if err != nil {
		return nil, err
	}
	var out []Creative
	return out, json.Unmarshal(b, &out)
}

// CreateCreativeInput is the payload for creating a standard creative that
// references an existing post/share.
type CreateCreativeInput struct {
	Campaign       string           `json:"campaign"`
	IntendedStatus string           `json:"intendedStatus"`
	Content        *CreativeContent `json:"content,omitempty"`
	Name           string           `json:"name,omitempty"`
}

// CreativeContent wraps the reference URN to an existing post/share.
type CreativeContent struct {
	Reference string `json:"reference"`
}

// CreateCreative creates a new creative under the given account. Returns the
// new resource id from the X-RestLi-Id header.
func CreateCreative(ctx context.Context, c *client.Client, accountID string, in *CreateCreativeInput) (string, error) {
	return c.PostJSON(ctx, "/adAccounts/"+accountID+"/creatives", in, nil)
}

// UpdateCreativeStatus performs a PARTIAL_UPDATE on a creative to change its
// intendedStatus. creativeURN is the full URN (urn:li:sponsoredCreative:123)
// or a bare numeric id.
func UpdateCreativeStatus(ctx context.Context, c *client.Client, accountID, creativeURN, status string) error {
	encoded := url.PathEscape(urn.Wrap(urn.Creative, creativeURN))
	body := map[string]any{
		"patch": map[string]any{
			"$set": map[string]any{
				"intendedStatus": status,
			},
		},
	}
	return c.PartialUpdate(ctx, "/adAccounts/"+accountID+"/creatives/"+encoded, body)
}

// GetCreative fetches a single creative by its bare numeric id. The URN is
// built and URL-encoded before placing it in the path segment.
func GetCreative(ctx context.Context, c *client.Client, accountID, creativeID string) (*Creative, error) {
	encodedURN := url.PathEscape(urn.Wrap(urn.Creative, creativeID))
	var cr Creative
	if err := c.GetJSON(ctx, "/adAccounts/"+accountID+"/creatives/"+encodedURN, nil, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}
