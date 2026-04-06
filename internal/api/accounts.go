// Package api wraps LinkedIn Marketing API resources behind small,
// strongly-typed helpers. Each function takes a *client.Client and returns
// decoded structs so that command-layer code stays free of HTTP plumbing.
package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

// Account is a LinkedIn sponsored ad account, decoded from /adAccounts.
type Account struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

// ListAccounts returns all ad accounts accessible by the current token.
// If limit > 0, iteration stops after limit items.
func ListAccounts(ctx context.Context, c *client.Client, limit int) ([]Account, error) {
	q := url.Values{}
	q.Set("q", "search")
	var out []Account
	if err := client.PaginateStartCount(ctx, c, "/adAccounts", q, 500, limit, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAccount fetches a single ad account by id.
func GetAccount(ctx context.Context, c *client.Client, id string) (*Account, error) {
	var a Account
	if err := c.GetJSON(ctx, "/adAccounts/"+id, nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}
