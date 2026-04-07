// Package resolve maps LinkedIn URNs to human-readable display names. It
// caches lookups in-process so a single CLI run that surfaces the same URN
// multiple times only pays for one HTTP round-trip per URN.
package resolve

import (
	"context"
	"fmt"
	"io"
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
	logger    io.Writer

	mu    sync.RWMutex
	cache map[string]entry
}

// New constructs a Resolver bound to the given HTTP client and account. The
// accountID is used to build nested paths for campaign groups and campaigns.
// The client may be nil for tests that only exercise the URN-fallback path.
func New(c *client.Client, accountID string) *Resolver {
	return &Resolver{client: c, accountID: accountID, ttl: defaultTTL, cache: map[string]entry{}}
}

// SetLogger attaches a writer to the resolver. When non-nil, every Resolve
// call writes a one-line trace ("resolve: <urn> → <name> (cached)" or
// "(fetched 120ms)") so callers can audit lookup activity under --verbose.
func (r *Resolver) SetLogger(w io.Writer) {
	r.logger = w
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
		r.log("resolve: %s → %s (cached)\n", urn, e.name)
		return e.name
	}
	r.mu.RUnlock()

	started := time.Now()
	name := r.fetch(ctx, urn)
	dur := time.Since(started)
	if name != urn {
		// Only cache successful lookups. When fetch fails it returns the
		// original URN; caching that would suppress retries for r.ttl.
		r.mu.Lock()
		r.cache[urn] = entry{name: name, expiresAt: time.Now().Add(r.ttl)}
		r.mu.Unlock()
		r.log("resolve: %s → %s (fetched %dms)\n", urn, name, dur.Milliseconds())
	} else {
		r.log("resolve: %s → (miss) (%dms)\n", urn, dur.Milliseconds())
	}
	return name
}

// log writes a one-line trace to the configured logger if any.
func (r *Resolver) log(format string, args ...any) {
	if r.logger == nil {
		return
	}
	_, _ = fmt.Fprintf(r.logger, format, args...)
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
//
// Targeting URNs (title/geo/organization/adSegment) speak response shapes that
// are genuinely unpredictable across LinkedIn's API versions — fetch decodes
// into map[string]any and probes a set of common name fields. Locale and
// staffCountRange URNs are parsed locally with no HTTP round-trip.
func (r *Resolver) fetch(ctx context.Context, urn string) string {
	// Purely-local (no-HTTP) kinds first — these work even when client is nil.
	switch {
	case strings.HasPrefix(urn, "urn:li:locale:"):
		if name := parseLocaleURN(urn); name != "" {
			return name
		}
		return urn
	case strings.HasPrefix(urn, "urn:li:staffCountRange:"):
		if name := parseStaffCountRangeURN(urn); name != "" {
			return name
		}
		return urn
	}

	if r.client == nil {
		return urn
	}
	id := lastSegment(urn)
	if id == "" {
		return urn
	}

	// Sponsored (account-scoped) URNs use typed decode — endpoints are stable
	// and always carry a top-level "name" field.
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
	}
	if path != "" {
		var body struct {
			Name string `json:"name"`
		}
		if err := r.client.GetJSON(ctx, path, nil, &body); err != nil || body.Name == "" {
			return urn
		}
		return body.Name
	}

	// Targeting URNs: response shapes vary, so probe a handful of common name
	// fields out of a generic map.
	switch {
	case strings.HasPrefix(urn, "urn:li:title:"):
		return probeName(ctx, r.client, "/titles/"+id, "locale=(country:US,language:en)", urn)
	case strings.HasPrefix(urn, "urn:li:geo:"):
		return probeName(ctx, r.client, "/geo/"+id, "locale=(country:US,language:en)", urn)
	case strings.HasPrefix(urn, "urn:li:adSegment:"):
		// dmpSegments 403s on most tokens — fallback handles that.
		return probeName(ctx, r.client, "/dmpSegments/"+id, "", urn)
	case strings.HasPrefix(urn, "urn:li:organization:"):
		return probeName(ctx, r.client, "/organizations/"+id, "projection=(localizedName)", urn)
	}
	return urn
}

// probeName fetches path (optionally with a raw query) and tries a handful of
// common name fields (name, localizedName, defaultLocalizedName.value, title)
// on the decoded body. Returns fallback on any failure. The rawQuery argument
// is forwarded verbatim, so LinkedIn's tuple-syntax parameters (e.g.
// "locale=(country:US,language:en)") reach the server unescaped.
func probeName(ctx context.Context, c *client.Client, path, rawQuery, fallback string) string {
	var body map[string]any
	var err error
	if rawQuery != "" {
		err = c.GetJSONRawQuery(ctx, path, rawQuery, &body)
	} else {
		err = c.GetJSON(ctx, path, nil, &body)
	}
	if err != nil || len(body) == 0 {
		return fallback
	}
	if name := extractName(body); name != "" {
		return name
	}
	return fallback
}

// extractName walks a decoded JSON object trying a few common name field
// shapes LinkedIn uses across endpoints. Order is: "name", "localizedName",
// "defaultLocalizedName.value", "title". The "name" field may itself be a
// locale map ({"en_US": "Software Engineer"}); that is handled as well.
func extractName(body map[string]any) string {
	if v, ok := body["name"]; ok {
		switch t := v.(type) {
		case string:
			if t != "" {
				return t
			}
		case map[string]any:
			if s := pickLocalizedString(t); s != "" {
				return s
			}
		}
	}
	if s, ok := body["localizedName"].(string); ok && s != "" {
		return s
	}
	if dln, ok := body["defaultLocalizedName"].(map[string]any); ok {
		if s, ok := dln["value"].(string); ok && s != "" {
			return s
		}
	}
	if s, ok := body["title"].(string); ok && s != "" {
		return s
	}
	return ""
}

// pickLocalizedString prefers en_US then falls back to any non-empty value in
// a {"en_US": "..."} style locale map.
func pickLocalizedString(m map[string]any) string {
	if s, ok := m["en_US"].(string); ok && s != "" {
		return s
	}
	for _, v := range m {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// parseLocaleURN maps "urn:li:locale:en_US" to "en_US". Returns "" when the
// URN is malformed so callers fall back to the URN itself.
func parseLocaleURN(urn string) string {
	const prefix = "urn:li:locale:"
	code := strings.TrimPrefix(urn, prefix)
	if code == "" || code == urn {
		return ""
	}
	return code
}

// parseStaffCountRangeURN maps "urn:li:staffCountRange:(1,10)" to "1-10
// employees" and the open-ended "(10001,)" form to "10001+ employees".
// Returns "" on any parse failure.
func parseStaffCountRangeURN(urn string) string {
	const prefix = "urn:li:staffCountRange:"
	rest := strings.TrimPrefix(urn, prefix)
	if rest == urn || !strings.HasPrefix(rest, "(") || !strings.HasSuffix(rest, ")") {
		return ""
	}
	inner := rest[1 : len(rest)-1]
	lo, hi, ok := strings.Cut(inner, ",")
	if !ok || lo == "" {
		return ""
	}
	if hi == "" {
		return lo + "+ employees"
	}
	return lo + "-" + hi + " employees"
}

// lastSegment returns the substring after the final ':' separator. For
// "urn:li:sponsoredCampaign:42" this is "42". Note this does not handle
// LinkedIn's tuple-suffix URNs (e.g. staffCountRange:(1,10)); those are
// parsed by dedicated helpers before lastSegment is consulted.
func lastSegment(urn string) string {
	i := strings.LastIndex(urn, ":")
	if i < 0 {
		return urn
	}
	return urn[i+1:]
}
