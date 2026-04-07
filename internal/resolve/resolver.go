// Package resolve maps LinkedIn URNs to human-readable display names. It
// caches lookups in-process so a single CLI run that surfaces the same URN
// multiple times only pays for one HTTP round-trip per URN.
package resolve

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

// defaultTTL bounds how long a cached name is considered fresh. Five minutes
// is short enough that renames during long sessions are eventually picked up
// and long enough that bursts of resolves stay free.
const defaultTTL = 5 * time.Minute

type entry struct {
	name      string
	expiresAt time.Time
}

// Resolver maps LinkedIn URNs to human-readable names. Safe for concurrent
// use; the underlying cache is guarded by a sync.RWMutex so reads are cheap
// and concurrent.
type Resolver struct {
	client    *client.Client
	accountID string
	ttl       time.Duration

	mu    sync.RWMutex
	cache map[string]entry
}

// New constructs a Resolver bound to the given HTTP client and account. The
// accountID is used to build nested paths for campaign groups and campaigns.
// The client may be nil for tests that only exercise the URN-fallback path.
func New(c *client.Client, accountID string) *Resolver {
	return &Resolver{client: c, accountID: accountID, ttl: defaultTTL, cache: map[string]entry{}}
}

// Resolve looks up the display name for a URN. On cache miss it issues the
// appropriate GET, caches the result, and returns it. On any failure it falls
// back to the URN itself so callers can render something meaningful instead
// of an empty string.
func (r *Resolver) Resolve(ctx context.Context, urn string) string {
	if urn == "" {
		return ""
	}
	r.mu.RLock()
	if e, ok := r.cache[urn]; ok && time.Now().Before(e.expiresAt) {
		r.mu.RUnlock()
		return e.name
	}
	r.mu.RUnlock()

	name := r.fetch(ctx, urn)
	if name != urn {
		// Only cache successful lookups. When fetch fails it returns the
		// original URN; caching that would suppress retries for r.ttl.
		r.mu.Lock()
		r.cache[urn] = entry{name: name, expiresAt: time.Now().Add(r.ttl)}
		r.mu.Unlock()
	}
	return name
}

// ResolveAll resolves a batch of URNs in parallel and returns a map of urn->
// name. Duplicates and empty strings are skipped before fan-out.
func (r *Resolver) ResolveAll(ctx context.Context, urns []string) map[string]string {
	out := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	seen := map[string]bool{}
	for _, u := range urns {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			n := r.Resolve(ctx, u)
			mu.Lock()
			out[u] = n
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	return out
}

// fetch dispatches a single URN to the right LinkedIn endpoint and returns
// the display name. Any failure (no client, unknown kind, HTTP error, missing
// name field) returns the URN unchanged so callers can fall back gracefully.
func (r *Resolver) fetch(ctx context.Context, urn string) string {
	if r.client == nil {
		return urn
	}
	id := lastSegment(urn)
	if id == "" {
		return urn
	}
	var path string
	switch {
	case strings.Contains(urn, ":sponsoredAccount:"):
		path = "/adAccounts/" + id
	case strings.Contains(urn, ":sponsoredCampaignGroup:"):
		if r.accountID == "" {
			return urn
		}
		path = "/adAccounts/" + r.accountID + "/adCampaignGroups/" + id
	case strings.Contains(urn, ":sponsoredCampaign:"):
		if r.accountID == "" {
			return urn
		}
		path = "/adAccounts/" + r.accountID + "/adCampaigns/" + id
	case strings.Contains(urn, ":sponsoredCreative:"):
		// Creatives don't carry a "name" field -- surfacing the URN is the
		// honest answer here.
		return urn
	default:
		return urn
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := r.client.GetJSON(ctx, path, nil, &body); err != nil || body.Name == "" {
		return urn
	}
	return body.Name
}

// lastSegment returns the substring after the final ':' separator. For
// "urn:li:sponsoredCampaign:42" this is "42".
func lastSegment(urn string) string {
	i := strings.LastIndex(urn, ":")
	if i < 0 {
		return urn
	}
	return urn[i+1:]
}
