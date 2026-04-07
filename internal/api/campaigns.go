package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// Campaign is a LinkedIn ad campaign decoded from
// /adAccounts/{accountId}/adCampaigns.
type Campaign struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	Status             string             `json:"status"`
	Account            string             `json:"account"`
	CampaignGroup      string             `json:"campaignGroup"`
	Type               string             `json:"type"`
	Objective          string             `json:"objectiveType"`
	Locale             *Locale            `json:"locale,omitempty"`
	DailyBudget        *Money             `json:"dailyBudget,omitempty"`
	TotalBudget        *Money             `json:"totalBudget,omitempty"`
	RunSchedule        *DateRange         `json:"runSchedule,omitempty"`
	CostType           string             `json:"costType"`
	UnitCost           *Money             `json:"unitCost,omitempty"`
	TargetingCriteria  *TargetingCriteria `json:"targetingCriteria,omitempty"`
	OptimizationTarget string             `json:"optimizationTargetType,omitempty"`
	ServingStatuses    []string           `json:"servingStatuses,omitempty"`
	Format             string             `json:"format,omitempty"`
}

// TargetingCriteria mirrors the LinkedIn targeting structure. Include uses the
// AND-of-OR shape (list of clauses, each with a facet→values "or" map).
// Exclude is a single facet→values map under "or".
type TargetingCriteria struct {
	Include *TargetingInclude `json:"include,omitempty"`
	Exclude *TargetingClause  `json:"exclude,omitempty"`
}

// TargetingInclude is a list of AND-ed TargetingClauses.
type TargetingInclude struct {
	And []TargetingClause `json:"and,omitempty"`
}

// TargetingClause is a facet→values map wrapped under "or".
type TargetingClause struct {
	Or map[string][]string `json:"or"`
}

// IncludedFacets flattens all include clauses into a single facet→values map.
// Multiple clauses on the same facet are concatenated.
func (t *TargetingCriteria) IncludedFacets() map[string][]string {
	out := map[string][]string{}
	if t == nil || t.Include == nil {
		return out
	}
	for _, c := range t.Include.And {
		for facet, vals := range c.Or {
			out[facet] = append(out[facet], vals...)
		}
	}
	return out
}

// ExcludedFacets returns the exclude side as a facet→values map.
func (t *TargetingCriteria) ExcludedFacets() map[string][]string {
	if t == nil || t.Exclude == nil {
		return map[string][]string{}
	}
	return t.Exclude.Or
}

// ListCampaigns returns campaigns under the given account id (bare). When
// groupID is non-empty, results are additionally filtered by campaign group
// using flat dot-notation search params. Uses PaginateToken.
// If limit > 0, iteration stops after limit items.
func ListCampaigns(ctx context.Context, c *client.Client, accountID, groupID string, limit int) ([]Campaign, error) {
	q := url.Values{}
	q.Set("q", "search")
	if groupID != "" {
		groupURN := urn.Wrap(urn.CampaignGroup, groupID)
		q.Set("search.campaignGroup.values[0]", groupURN)
	}
	var out []Campaign
	if err := client.PaginateToken(ctx, c, "/adAccounts/"+accountID+"/adCampaigns", q, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCampaign fetches a single campaign by id under accountID.
func GetCampaign(ctx context.Context, c *client.Client, accountID, id string) (*Campaign, error) {
	var camp Campaign
	if err := c.GetJSON(ctx, "/adAccounts/"+accountID+"/adCampaigns/"+id, nil, &camp); err != nil {
		return nil, err
	}
	return &camp, nil
}

// CreateCampaignInput is the request body for
// POST /adAccounts/{accountId}/adCampaigns. Account and CampaignGroup must be
// full URNs.
type CreateCampaignInput struct {
	Account       string     `json:"account"`
	CampaignGroup string     `json:"campaignGroup"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	Type          string     `json:"type"`
	ObjectiveType string     `json:"objectiveType"`
	CostType      string     `json:"costType"`
	Locale        *Locale    `json:"locale,omitempty"`
	DailyBudget   *Money     `json:"dailyBudget,omitempty"`
	TotalBudget   *Money     `json:"totalBudget,omitempty"`
	UnitCost      *Money     `json:"unitCost,omitempty"`
	RunSchedule   *DateRange `json:"runSchedule,omitempty"`
}

// CreateCampaign creates a new campaign under accountID and returns the new id.
// Status defaults to DRAFT and CostType to CPM when unset.
func CreateCampaign(ctx context.Context, c *client.Client, accountID string, in *CreateCampaignInput) (string, error) {
	if in.Status == "" {
		in.Status = "DRAFT"
	}
	if in.CostType == "" {
		in.CostType = "CPM"
	}
	return c.PostJSON(ctx, "/adAccounts/"+accountID+"/adCampaigns", in, nil)
}

// UpdateCampaignInput is the partial-update body for a campaign. Only Status,
// Name, DailyBudget and UnitCost (bid) can be touched via this CLI.
type UpdateCampaignInput struct {
	Status      *string `json:"status,omitempty"`
	Name        *string `json:"name,omitempty"`
	DailyBudget *Money  `json:"dailyBudget,omitempty"`
	UnitCost    *Money  `json:"unitCost,omitempty"`
}

// UpdateCampaign applies a partial update to a campaign via the Rest.li
// PARTIAL_UPDATE protocol.
func UpdateCampaign(ctx context.Context, c *client.Client, accountID, id string, in *UpdateCampaignInput) error {
	body := map[string]any{
		"patch": map[string]any{"$set": in},
	}
	return c.PartialUpdate(ctx, "/adAccounts/"+accountID+"/adCampaigns/"+id, body)
}

// DeleteCampaign hard-deletes a campaign by id. Per LinkedIn semantics, only
// DRAFT campaigns can be hard-deleted; non-draft campaigns must be
// soft-deleted via UpdateCampaign with status PENDING_DELETION. The dispatch
// is handled at the cmd layer.
func DeleteCampaign(ctx context.Context, c *client.Client, accountID, id string) error {
	return c.Delete(ctx, "/adAccounts/"+accountID+"/adCampaigns/"+id)
}
