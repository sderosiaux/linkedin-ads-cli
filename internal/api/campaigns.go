package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Campaign is a LinkedIn ad campaign decoded from /adCampaigns.
type Campaign struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	Account       string     `json:"account"`
	CampaignGroup string     `json:"campaignGroup"`
	Type          string     `json:"type"`
	Objective     string     `json:"objectiveType"`
	Locale        *Locale    `json:"locale,omitempty"`
	DailyBudget   *Money     `json:"dailyBudget,omitempty"`
	TotalBudget   *Money     `json:"totalBudget,omitempty"`
	RunSchedule   *DateRange `json:"runSchedule,omitempty"`
	CostType      string     `json:"costType"`
	UnitCost      *Money     `json:"unitCost,omitempty"`
}

// ListCampaigns returns campaigns under the given account id (bare). When
// groupID is non-empty, results are additionally filtered by campaign group.
// If limit > 0, iteration stops after limit items.
func ListCampaigns(ctx context.Context, c *client.Client, accountID, groupID string, limit int) ([]Campaign, error) {
	accountURN := urn.Wrap(urn.Account, accountID)
	search := fmt.Sprintf("(account:(values:List(%s))", accountURN)
	if groupID != "" {
		groupURN := urn.Wrap(urn.CampaignGroup, groupID)
		search += fmt.Sprintf(",campaignGroup:(values:List(%s))", groupURN)
	}
	search += ")"

	q := url.Values{}
	q.Set("q", "search")
	q.Set("search", search)
	var out []Campaign
	if err := client.PaginateStartCount(ctx, c, "/adCampaigns", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCampaign fetches a single campaign by id.
func GetCampaign(ctx context.Context, c *client.Client, id string) (*Campaign, error) {
	var camp Campaign
	if err := c.GetJSON(ctx, "/adCampaigns/"+id, nil, &camp); err != nil {
		return nil, err
	}
	return &camp, nil
}
