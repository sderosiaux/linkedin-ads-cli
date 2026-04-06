package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// LeadForm is a LinkedIn lead generation form decoded from /leadGenForms.
type LeadForm struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Account        string  `json:"account"`
	Locale         *Locale `json:"locale,omitempty"`
	Headline       string  `json:"headline,omitempty"`
	Description    string  `json:"description,omitempty"`
	CreatedAt      int64   `json:"createdAt,omitempty"`
	LastModifiedAt int64   `json:"lastModifiedAt,omitempty"`
}

// ListLeadForms returns lead-gen forms under the given account id (bare).
func ListLeadForms(ctx context.Context, c *client.Client, accountID string, limit int) ([]LeadForm, error) {
	q := url.Values{}
	q.Set("q", "account")
	q.Set("account", urn.Wrap(urn.Account, accountID))
	var out []LeadForm
	if err := client.PaginateStartCount(ctx, c, "/leadGenForms", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}
