package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

// TestDryRun_NoHTTPCalls is the cross-cutting safety net for --dry-run.
// Every write subcommand must short-circuit before any write HTTP request, no
// matter what its individual unit tests check. We hand them a server that
// allows GETs (update commands need to fetch the current state to render a
// diff) and t.Fatal()s on any other method, then assert each command still
// produces a human-readable preview.
func TestDryRun_NoHTTPCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/adAccounts/12345/adCampaignGroups/111":
				_, _ = w.Write([]byte(`{"id":111,"name":"Q1","status":"PAUSED","account":"urn:li:sponsoredAccount:12345"}`))
			case "/adCampaigns/10":
				_, _ = w.Write([]byte(`{"id":10,"name":"X","status":"PAUSED","account":"urn:li:sponsoredAccount:12345","campaignGroup":"urn:li:sponsoredCampaignGroup:111","type":"SPONSORED_UPDATES","objectiveType":"WEBSITE_VISIT","costType":"CPC"}`))
			default:
				t.Errorf("unexpected GET %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		t.Fatalf("dry-run should make no write HTTP calls, but received %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{ //nolint:gosec // test fixture, not a real token
		Token:          "x",
		DefaultAccount: "12345",
		APIVersion:     "202601",
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "campaign-groups create",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaign-groups", "create",
				"--name", "Test", "--total-budget", "5000", "--currency", "USD",
			},
			want: "POST /adAccounts/12345/adCampaignGroups",
		},
		{
			name: "campaign-groups update",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaign-groups", "update", "111", "--status", "ACTIVE",
			},
			want: "POST /adAccounts/12345/adCampaignGroups/111",
		},
		{
			name: "campaign-groups delete",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaign-groups", "delete", "111",
			},
			want: "DELETE /adAccounts/12345/adCampaignGroups/111",
		},
		{
			name: "campaigns create",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaigns", "create",
				"--group", "678", "--name", "Test", "--daily-budget", "100",
				"--currency", "USD", "--objective", "BRAND_AWARENESS",
				"--type", "SPONSORED_UPDATES",
			},
			want: "POST /adCampaigns",
		},
		{
			name: "campaigns update",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaigns", "update", "10", "--status", "ACTIVE",
			},
			want: "POST /adCampaigns/10",
		},
		{
			name: "campaigns delete",
			args: []string{
				"--config", cfgPath, "--dry-run",
				"campaigns", "delete", "10",
			},
			want: "DELETE /adCampaigns/10",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := NewRootCmd()
			out := &bytes.Buffer{}
			root.SetOut(out)
			root.SetErr(out)
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v\noutput: %s", err, out.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("expected %q in dry-run output, got: %s", tc.want, out.String())
			}
		})
	}
}
