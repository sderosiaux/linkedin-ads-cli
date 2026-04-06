package cmd

import "github.com/sderosiaux/linkedin-ads-cli/internal/api"

// compactAccount projects an Account to the spec whitelist (id, name, status,
// type, currency).
func compactAccount(v any) any {
	a := v.(api.Account)
	return struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Status   string `json:"status"`
		Type     string `json:"type"`
		Currency string `json:"currency"`
	}{a.ID, a.Name, a.Status, a.Type, a.Currency}
}

// compactCampaignGroup projects a CampaignGroup to the spec whitelist (id,
// name, status, totalBudget, runSchedule).
func compactCampaignGroup(v any) any {
	g := v.(api.CampaignGroup)
	return struct {
		ID          int64          `json:"id"`
		Name        string         `json:"name"`
		Status      string         `json:"status"`
		TotalBudget *api.Money     `json:"totalBudget,omitempty"`
		RunSchedule *api.DateRange `json:"runSchedule,omitempty"`
	}{g.ID, g.Name, g.Status, g.TotalBudget, g.RunSchedule}
}

// compactCampaign projects a Campaign to the spec whitelist (id, name, status,
// campaignGroup, dailyBudget, objectiveType).
func compactCampaign(v any) any {
	c := v.(api.Campaign)
	return struct {
		ID            int64      `json:"id"`
		Name          string     `json:"name"`
		Status        string     `json:"status"`
		CampaignGroup string     `json:"campaignGroup"`
		DailyBudget   *api.Money `json:"dailyBudget,omitempty"`
		ObjectiveType string     `json:"objectiveType,omitempty"`
	}{c.ID, c.Name, c.Status, c.CampaignGroup, c.DailyBudget, c.Objective}
}

// compactCreative projects a Creative to the spec whitelist (id, status,
// intendedStatus, campaign, review).
func compactCreative(v any) any {
	cr := v.(api.Creative)
	return struct {
		ID             string              `json:"id"`
		Status         string              `json:"status"`
		IntendedStatus string              `json:"intendedStatus"`
		Campaign       string              `json:"campaign"`
		Review         *api.CreativeReview `json:"review,omitempty"`
	}{cr.ID, cr.Status, cr.IntendedStatus, cr.Campaign, cr.Review}
}

// compactAnalyticsRow projects an AnalyticsRow to the spec whitelist
// (dateRange, impressions, clicks, costInUsd, conversions). The conversions
// field maps to externalWebsiteConversions on the wire to keep API parity.
func compactAnalyticsRow(v any) any {
	r := v.(api.AnalyticsRow)
	return struct {
		DateRange   map[string]any `json:"dateRange,omitempty"`
		Impressions int64          `json:"impressions"`
		Clicks      int64          `json:"clicks"`
		CostInUsd   string         `json:"costInUsd"`
		Conversions int64          `json:"externalWebsiteConversions,omitempty"`
	}{r.DateRange, r.Impressions, r.Clicks, r.CostInUsd, r.Conversions}
}
