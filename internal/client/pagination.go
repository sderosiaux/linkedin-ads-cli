package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// pagedResponse is the Rest.li start/count envelope returned by most LinkedIn
// marketing finders. Only the fields we consume are decoded.
type pagedResponse struct {
	Elements json.RawMessage `json:"elements"`
	Paging   struct {
		Start int `json:"start"`
		Count int `json:"count"`
		Total int `json:"total"`
	} `json:"paging"`
}

// PaginateStartCount walks a Rest.li finder using start/count pagination and
// decodes the concatenated elements into dst (which must be a pointer to a
// slice). If limit > 0, iteration stops as soon as dst holds limit items.
// pageSize defaults to 500 when non-positive.
func PaginateStartCount(ctx context.Context, c *Client, path string, q url.Values, pageSize, limit int, dst any) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	if q == nil {
		q = url.Values{}
	}
	start := 0
	var accumulated []json.RawMessage
	for {
		q.Set("start", strconv.Itoa(start))
		q.Set("count", strconv.Itoa(pageSize))

		var page pagedResponse
		if err := c.GetJSON(ctx, path, q, &page); err != nil {
			return err
		}
		var raws []json.RawMessage
		if len(page.Elements) > 0 {
			if err := json.Unmarshal(page.Elements, &raws); err != nil {
				return err
			}
		}
		accumulated = append(accumulated, raws...)

		if limit > 0 && len(accumulated) >= limit {
			accumulated = accumulated[:limit]
			break
		}
		if len(raws) < pageSize {
			break
		}
		if page.Paging.Total > 0 && start+pageSize >= page.Paging.Total {
			break
		}
		start += pageSize
	}

	b, err := json.Marshal(accumulated)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// pagedTokenResponse is the cursor envelope used by newer LinkedIn finders.
type pagedTokenResponse struct {
	Elements json.RawMessage `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

// PaginateToken walks an endpoint using the metadata.nextPageToken cursor and
// decodes the concatenated elements into dst. If limit > 0, iteration stops as
// soon as dst holds limit items.
func PaginateToken(ctx context.Context, c *Client, path string, q url.Values, limit int, dst any) error {
	if q == nil {
		q = url.Values{}
	}
	var accumulated []json.RawMessage
	for {
		var page pagedTokenResponse
		if err := c.GetJSON(ctx, path, q, &page); err != nil {
			return err
		}
		var raws []json.RawMessage
		if len(page.Elements) > 0 {
			if err := json.Unmarshal(page.Elements, &raws); err != nil {
				return err
			}
		}
		accumulated = append(accumulated, raws...)

		if limit > 0 && len(accumulated) >= limit {
			accumulated = accumulated[:limit]
			break
		}
		if page.Metadata.NextPageToken == "" {
			break
		}
		q.Set("pageToken", page.Metadata.NextPageToken)
	}

	b, err := json.Marshal(accumulated)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
