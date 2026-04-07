package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestResolverCachesAndFetches(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = calls.Add(1)
		if strings.HasPrefix(r.URL.Path, "/adAccounts/777/adCampaigns/") {
			_, _ = w.Write([]byte(`{"id":42,"name":"Spring Promo","status":"ACTIVE"}`))
		}
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	n1 := r.Resolve(context.Background(), "urn:li:sponsoredCampaign:42")
	n2 := r.Resolve(context.Background(), "urn:li:sponsoredCampaign:42")
	if n1 != "Spring Promo" || n2 != "Spring Promo" {
		t.Errorf("name: %q / %q", n1, n2)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 fetch, got %d", calls.Load())
	}
}

func TestResolverGracefulOnFetchError(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	got := r.Resolve(context.Background(), "urn:li:sponsoredCampaign:1")
	if got != "urn:li:sponsoredCampaign:1" {
		t.Errorf("expected URN as fallback, got %q", got)
	}
	// Failed lookups must not be cached — a second call should retry.
	got2 := r.Resolve(context.Background(), "urn:li:sponsoredCampaign:1")
	if got2 != "urn:li:sponsoredCampaign:1" {
		t.Errorf("second call: expected URN fallback, got %q", got2)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 fetches (no negative cache), got %d", calls.Load())
	}
}

func TestResolveAllParallel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/adCampaignGroups/"):
			_, _ = w.Write([]byte(`{"id":1,"name":"Group One"}`))
		case strings.Contains(r.URL.Path, "/adCampaigns/"):
			_, _ = w.Write([]byte(`{"id":2,"name":"Camp Two"}`))
		}
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	out := r.ResolveAll(context.Background(), []string{
		"urn:li:sponsoredCampaignGroup:1",
		"urn:li:sponsoredCampaign:2",
		"urn:li:sponsoredCampaignGroup:1", // duplicate
	})
	if out["urn:li:sponsoredCampaignGroup:1"] != "Group One" {
		t.Errorf("group: %v", out)
	}
	if out["urn:li:sponsoredCampaign:2"] != "Camp Two" {
		t.Errorf("camp: %v", out)
	}
}

func TestResolveAccount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/adAccounts/") {
			_, _ = w.Write([]byte(`{"id":777,"name":"Acme EMEA","currency":"USD"}`))
		}
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	got := r.Resolve(context.Background(), "urn:li:sponsoredAccount:777")
	if got != "Acme EMEA" {
		t.Errorf("expected Acme EMEA, got %q", got)
	}
}

func TestResolveEmptyURNReturnsEmpty(t *testing.T) {
	t.Parallel()
	r := New(nil, "")
	if got := r.Resolve(context.Background(), ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveUnknownKindFallsBack(t *testing.T) {
	t.Parallel()
	r := New(nil, "")
	urn := "urn:li:something:42"
	if got := r.Resolve(context.Background(), urn); got != urn {
		t.Errorf("expected URN fallback, got %q", got)
	}
}

func TestParseLocaleURN(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"urn:li:locale:en_US", "en_US"},
		{"urn:li:locale:fr_FR", "fr_FR"},
		{"urn:li:locale:", ""},
		{"urn:li:other:en_US", ""},
	}
	for _, tc := range cases {
		if got := parseLocaleURN(tc.in); got != tc.want {
			t.Errorf("parseLocaleURN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseStaffCountRangeURN(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"urn:li:staffCountRange:(1,1)", "1-1 employees"},
		{"urn:li:staffCountRange:(2,10)", "2-10 employees"},
		{"urn:li:staffCountRange:(10001,)", "10001+ employees"},
		{"urn:li:staffCountRange:", ""},
		{"urn:li:staffCountRange:1-10", ""},   // missing parens
		{"urn:li:staffCountRange:(,10)", ""},  // empty lo
		{"urn:li:staffCountRange:(1-10)", ""}, // no comma
	}
	for _, tc := range cases {
		if got := parseStaffCountRangeURN(tc.in); got != tc.want {
			t.Errorf("parseStaffCountRangeURN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveLocaleAndStaffCountRange_NoHTTP(t *testing.T) {
	t.Parallel()
	// client nil — these URNs must resolve locally.
	r := New(nil, "")
	if got := r.Resolve(context.Background(), "urn:li:locale:en_US"); got != "en_US" {
		t.Errorf("locale: got %q", got)
	}
	if got := r.Resolve(context.Background(), "urn:li:staffCountRange:(2,10)"); got != "2-10 employees" {
		t.Errorf("staffCountRange: got %q", got)
	}
	if got := r.Resolve(context.Background(), "urn:li:staffCountRange:(10001,)"); got != "10001+ employees" {
		t.Errorf("staffCountRange open: got %q", got)
	}
}

func TestResolveTitle_HappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/titles/") {
			t.Errorf("path: %s", r.URL.Path)
		}
		// Verify the raw query wasn't percent-mangled.
		if !strings.Contains(r.URL.RawQuery, "(country:US,language:en)") {
			t.Errorf("raw query mangled: %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"id":11405,"name":{"en_US":"Software Engineer"}}`))
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	got := r.Resolve(context.Background(), "urn:li:title:11405")
	if got != "Software Engineer" {
		t.Errorf("expected 'Software Engineer', got %q", got)
	}
}

func TestResolveTitle_FallsBackOnEmptyBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":42}`)) // no name / localizedName / title
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	urn := "urn:li:title:42"
	if got := r.Resolve(context.Background(), urn); got != urn {
		t.Errorf("expected URN fallback on empty body, got %q", got)
	}
}

func TestResolveGeo_DefaultLocalizedName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":101174742,"defaultLocalizedName":{"value":"United States"}}`))
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	if got := r.Resolve(context.Background(), "urn:li:geo:101174742"); got != "United States" {
		t.Errorf("geo: got %q", got)
	}
}

func TestResolveOrganization_LocalizedName(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":1009,"localizedName":"Microsoft"}`))
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	if got := r.Resolve(context.Background(), "urn:li:organization:1009"); got != "Microsoft" {
		t.Errorf("org: got %q", got)
	}
}

func TestResolveAdSegment_403FallsBack(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c, "777")
	urn := "urn:li:adSegment:62755117"
	if got := r.Resolve(context.Background(), urn); got != urn {
		t.Errorf("expected URN fallback on 403, got %q", got)
	}
}

func TestResolver_Logger(t *testing.T) {
	r := New(nil, "")
	var b strings.Builder
	r.SetLogger(&b)
	// purely-local locale URN — works without an HTTP client
	_ = r.Resolve(context.Background(), "urn:li:locale:en_US")
	if !strings.Contains(b.String(), "resolve: urn:li:locale:en_US") {
		t.Errorf("expected log line, got: %q", b.String())
	}
	// second call should be cached
	b.Reset()
	_ = r.Resolve(context.Background(), "urn:li:locale:en_US")
	if !strings.Contains(b.String(), "(cached)") {
		t.Errorf("expected cached log line, got: %q", b.String())
	}
}
