package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListAudiences(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dmpSegments" {
			t.Errorf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "account" {
			t.Errorf("q: %q", q.Get("q"))
		}
		if q.Get("account") != "urn:li:sponsoredAccount:12345" {
			t.Errorf("account: %q", q.Get("account"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{
					"id":             1,
					"name":           "Lookalike A",
					"type":           "LOOKALIKE",
					"sourcePlatform": "LINKEDIN",
					"status":         "READY",
					"audienceCount":  10000,
					"matchedCount":   8000,
					"description":    "test",
				},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture, not a real token
	auds, err := ListAudiences(context.Background(), c, "12345", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(auds) != 1 {
		t.Fatalf("len: %d", len(auds))
	}
	a := auds[0]
	if a.ID != 1 || a.Name != "Lookalike A" || a.Status != "READY" {
		t.Errorf("audience: %+v", a)
	}
	if a.AudienceCount != 10000 || a.MatchedCount != 8000 {
		t.Errorf("counts: %+v", a)
	}
}
