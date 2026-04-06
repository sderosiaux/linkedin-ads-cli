package api

import (
	"context"
	"fmt"

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
	rawQuery := fmt.Sprintf("q=search&search=(account:(values:List(%s)))", accountURN)
	var out []CampaignGroup
	if err := client.PaginateStartCountRaw(ctx, c, "/adCampaignGroups", rawQuery, 500, limit, &out); err != nil {
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

// CreateCampaignGroupInput is the request body for POST /adCampaignGroups.
// Account must be a full URN.
type CreateCampaignGroupInput struct {
	Account     string     `json:"account"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	TotalBudget *Money     `json:"totalBudget,omitempty"`
	RunSchedule *DateRange `json:"runSchedule,omitempty"`
}

// CreateCampaignGroup creates a new campaign group and returns the new id.
// Status defaults to DRAFT when unset.
func CreateCampaignGroup(ctx context.Context, c *client.Client, in *CreateCampaignGroupInput) (string, error) {
	if in.Status == "" {
		in.Status = "DRAFT"
	}
	return c.PostJSON(ctx, "/adCampaignGroups", in, nil)
}

// UpdateCampaignGroupInput is the partial-update body for a campaign group.
// All fields are pointers so unset fields are omitted from the wire payload —
// LinkedIn's $set semantics treat absent keys as untouched.
type UpdateCampaignGroupInput struct {
	Status      *string    `json:"status,omitempty"`
	Name        *string    `json:"name,omitempty"`
	TotalBudget *Money     `json:"totalBudget,omitempty"`
	RunSchedule *DateRange `json:"runSchedule,omitempty"`
}

// UpdateCampaignGroup applies a partial update to a campaign group via the
// Rest.li PARTIAL_UPDATE protocol.
func UpdateCampaignGroup(ctx context.Context, c *client.Client, id string, in *UpdateCampaignGroupInput) error {
	body := map[string]any{
		"patch": map[string]any{"$set": in},
	}
	return c.PartialUpdate(ctx, "/adCampaignGroups/"+id, body)
}

// DeleteCampaignGroup deletes a campaign group by id.
func DeleteCampaignGroup(ctx context.Context, c *client.Client, id string) error {
	return c.Delete(ctx, "/adCampaignGroups/"+id)
}
