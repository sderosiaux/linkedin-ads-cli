package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// CampaignGroup is a LinkedIn ad campaign group, decoded from /adCampaignGroups.
type CampaignGroup struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	Account         string     `json:"account"`
	TotalBudget     *Money     `json:"totalBudget,omitempty"`
	RunSchedule     *DateRange `json:"runSchedule,omitempty"`
	ServingStatuses []string   `json:"servingStatuses,omitempty"`
}

// ListCampaignGroups returns campaign groups under the given account id (bare,
// not URN). If limit > 0, iteration stops after limit items.
func ListCampaignGroups(ctx context.Context, c *client.Client, accountID string, limit int) ([]CampaignGroup, error) {
	accountURN := urn.Wrap(urn.Account, accountID)
	q := url.Values{}
	q.Set("q", "search")
	q.Set("search", fmt.Sprintf("(account:(values:List(%s)))", accountURN))
	var out []CampaignGroup
	if err := client.PaginateStartCount(ctx, c, "/adCampaignGroups", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCampaignGroup fetches a single campaign group by id.
func GetCampaignGroup(ctx context.Context, c *client.Client, id string) (*CampaignGroup, error) {
	var g CampaignGroup
	if err := c.GetJSON(ctx, "/adCampaignGroups/"+id, nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
