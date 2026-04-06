package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

// CampaignGroup is a LinkedIn ad campaign group, decoded from
// /adAccounts/{accountId}/adCampaignGroups.
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
// not URN). The account scoping happens via the nested path; only optional
// search filters (e.g. status) ride along as flat dot-notation params.
// If limit > 0, iteration stops after limit items.
func ListCampaignGroups(ctx context.Context, c *client.Client, accountID string, limit int) ([]CampaignGroup, error) {
	q := url.Values{}
	q.Set("q", "search")
	var out []CampaignGroup
	if err := client.PaginateToken(ctx, c, "/adAccounts/"+accountID+"/adCampaignGroups", q, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCampaignGroup fetches a single campaign group by id under accountID.
func GetCampaignGroup(ctx context.Context, c *client.Client, accountID, id string) (*CampaignGroup, error) {
	var g CampaignGroup
	if err := c.GetJSON(ctx, "/adAccounts/"+accountID+"/adCampaignGroups/"+id, nil, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateCampaignGroupInput is the request body for
// POST /adAccounts/{accountId}/adCampaignGroups. Account must be a full URN.
type CreateCampaignGroupInput struct {
	Account     string     `json:"account"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	TotalBudget *Money     `json:"totalBudget,omitempty"`
	RunSchedule *DateRange `json:"runSchedule,omitempty"`
}

// CreateCampaignGroup creates a new campaign group under accountID and returns
// the new id. Status defaults to DRAFT when unset.
func CreateCampaignGroup(ctx context.Context, c *client.Client, accountID string, in *CreateCampaignGroupInput) (string, error) {
	if in.Status == "" {
		in.Status = "DRAFT"
	}
	return c.PostJSON(ctx, "/adAccounts/"+accountID+"/adCampaignGroups", in, nil)
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
func UpdateCampaignGroup(ctx context.Context, c *client.Client, accountID, id string, in *UpdateCampaignGroupInput) error {
	body := map[string]any{
		"patch": map[string]any{"$set": in},
	}
	return c.PartialUpdate(ctx, "/adAccounts/"+accountID+"/adCampaignGroups/"+id, body)
}

// DeleteCampaignGroup hard-deletes a campaign group by id. Per LinkedIn
// semantics, only DRAFT groups can be hard-deleted; non-draft groups must be
// soft-deleted via UpdateCampaignGroup with status PENDING_DELETION. The
// dispatch is handled at the cmd layer.
func DeleteCampaignGroup(ctx context.Context, c *client.Client, accountID, id string) error {
	return c.Delete(ctx, "/adAccounts/"+accountID+"/adCampaignGroups/"+id)
}
