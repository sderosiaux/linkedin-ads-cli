package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Audience is a DMP segment decoded from /dmpSegments.
type Audience struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	SourcePlatform string `json:"sourcePlatform,omitempty"`
	Status         string `json:"status"`
	AudienceCount  int64  `json:"audienceCount,omitempty"`
	MatchedCount   int64  `json:"matchedCount,omitempty"`
	Description    string `json:"description,omitempty"`
}

// ListAudiences returns DMP segments under the given account id (bare).
func ListAudiences(ctx context.Context, c *client.Client, accountID string, limit int) ([]Audience, error) {
	q := url.Values{}
	q.Set("q", "account")
	q.Set("account", urn.Wrap(urn.Account, accountID))
	var out []Audience
	if err := client.PaginateStartCount(ctx, c, "/dmpSegments", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}
