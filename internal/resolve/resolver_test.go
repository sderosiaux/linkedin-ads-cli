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
		if strings.HasPrefix(r.URL.Path, "/adCampaigns/") {
			_, _ = w.Write([]byte(`{"id":42,"name":"Spring Promo","status":"ACTIVE"}`))
		}
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	r := New(c)
	got := r.Resolve(context.Background(), "urn:li:sponsoredCampaign:1")
	if got != "urn:li:sponsoredCampaign:1" {
		t.Errorf("expected URN as fallback, got %q", got)
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
	r := New(c)
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
	r := New(c)
	got := r.Resolve(context.Background(), "urn:li:sponsoredAccount:777")
	if got != "Acme EMEA" {
		t.Errorf("expected Acme EMEA, got %q", got)
	}
}

func TestResolveEmptyURNReturnsEmpty(t *testing.T) {
	t.Parallel()
	r := New(nil)
	if got := r.Resolve(context.Background(), ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveUnknownKindFallsBack(t *testing.T) {
	t.Parallel()
	r := New(nil)
	urn := "urn:li:something:42"
	if got := r.Resolve(context.Background(), urn); got != urn {
		t.Errorf("expected URN fallback, got %q", got)
	}
}
