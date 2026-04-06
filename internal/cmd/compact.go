package cmd

import "github.com/sderosiaux/linkedin-ads-cli/internal/api"

// compactAccount projects an Account to the minimal set of fields users
// typically need from `accounts list`.
func compactAccount(v any) any {
	a := v.(api.Account)
	return struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Status   string `json:"status"`
		Currency string `json:"currency"`
	}{a.ID, a.Name, a.Status, a.Currency}
}

// compactCampaignGroup projects a CampaignGroup to id/name/status.
func compactCampaignGroup(v any) any {
	g := v.(api.CampaignGroup)
	return struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}{g.ID, g.Name, g.Status}
}

// compactCampaign projects a Campaign to id/name/status/campaignGroup.
func compactCampaign(v any) any {
	c := v.(api.Campaign)
	return struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Status        string `json:"status"`
		CampaignGroup string `json:"campaignGroup"`
	}{c.ID, c.Name, c.Status, c.CampaignGroup}
}
