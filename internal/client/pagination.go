package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// pagedResponse is the Rest.li start/count envelope returned by most LinkedIn
// marketing finders. Only the fields we consume are decoded.
type pagedResponse struct {
	Elements json.RawMessage `json:"elements"`
	Paging   struct {
		Start int `json:"start"`
		Count int `json:"count"`
		Total int `json:"total"`
		Links []struct {
			Rel  string `json:"rel"`
			Href string `json:"href"`
		} `json:"links"`
	} `json:"paging"`
}

// nextHref returns the href of the first paging.links entry whose rel is
// "next", or "" when no such link is present.
func (p *pagedResponse) nextHref() string {
	for _, l := range p.Paging.Links {
		if l.Rel == "next" {
			return l.Href
		}
	}
	return ""
}

// extractPathAndQuery turns an absolute paging.links[rel=next] href into the
// path+query that the client expects (relative to its configured base URL).
// Returns ok=false when the href points to a different host or cannot be
// parsed — callers fall back to start+count arithmetic in that case.
func extractPathAndQuery(href, base string) (path, rawQuery string, ok bool) {
	if href == "" {
		return "", "", false
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", "", false
	}
	bu, err := url.Parse(base)
	if err != nil {
		return "", "", false
	}
	if u.Host != bu.Host {
		return "", "", false
	}
	p := u.Path
	if bu.Path != "" && bu.Path != "/" {
		p = strings.TrimPrefix(p, bu.Path)
	}
	if p == "" {
		p = "/"
	}
	return p, u.RawQuery, true
}

// PaginateStartCount walks a Rest.li finder using start/count pagination and
// decodes the concatenated elements into dst (which must be a pointer to a
// slice). If limit > 0, iteration stops as soon as dst holds limit items.
// pageSize defaults to 500 when non-positive.
//
// When the response carries paging.links[rel=next], the iterator follows that
// href instead of incrementing start, matching the LinkedIn Rest.li 2.0
// pagination contract.
func PaginateStartCount(ctx context.Context, c *Client, path string, q url.Values, pageSize, limit int, dst any) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	if q == nil {
		q = url.Values{}
	}
	start := 0
	useArithmetic := true
	curPath := path
	curRawQuery := ""
	var accumulated []json.RawMessage
	for {
		var page pagedResponse
		if useArithmetic {
			q.Set("start", strconv.Itoa(start))
			q.Set("count", strconv.Itoa(pageSize))
			if err := c.GetJSON(ctx, curPath, q, &page); err != nil {
				return err
			}
		} else {
			if err := c.GetJSONRawQuery(ctx, curPath, curRawQuery, &page); err != nil {
				return err
			}
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
		if href := page.nextHref(); href != "" {
			if p, rq, ok := extractPathAndQuery(href, c.BaseURL()); ok {
				useArithmetic = false
				curPath = p
				curRawQuery = rq
				continue
			}
		}
		// Once we're following hrefs, the absence of a next link is the
		// authoritative end-of-stream signal — fall through to start+count
		// arithmetic only when we never saw a link in the first place.
		if !useArithmetic {
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

// PaginateStartCountRaw is like PaginateStartCount but accepts a raw query
// string instead of url.Values. Callers who build Rest.li finder clauses that
// contain parentheses or colons must use this to avoid percent-encoding them.
// The pagination cursor (start/count) is appended to the raw query by the
// iterator. pageSize defaults to 500 when non-positive.
//
// As with PaginateStartCount, the iterator follows paging.links[rel=next]
// when present.
func PaginateStartCountRaw(ctx context.Context, c *Client, path, rawQuery string, pageSize, limit int, dst any) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	start := 0
	curPath := path
	curRawQuery := rawQuery
	useExplicitNext := false
	var accumulated []json.RawMessage
	for {
		fullRawQuery := curRawQuery
		if !useExplicitNext {
			fullRawQuery = fmt.Sprintf("%s&start=%d&count=%d", curRawQuery, start, pageSize)
		}

		var page pagedResponse
		if err := c.GetJSONRawQuery(ctx, curPath, fullRawQuery, &page); err != nil {
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
		if href := page.nextHref(); href != "" {
			if p, rq, ok := extractPathAndQuery(href, c.BaseURL()); ok {
				useExplicitNext = true
				curPath = p
				curRawQuery = rq
				continue
			}
		}
		// Once we're following hrefs, the absence of a next link is the
		// authoritative end-of-stream signal.
		if useExplicitNext {
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
