package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestConversionsList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 1, "name": "Signup", "type": "LANDING", "enabled": true, "attributionType": "LAST_TOUCH_BY_CAMPAIGN", "account": "urn:li:sponsoredAccount:777"},
			},
			"paging": map[string]any{"start": 0, "count": 1, "total": 1},
		})
	}))
	defer srv.Close()

	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "--json", "conversions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": 1`) {
		t.Errorf("got: %s", out.String())
	}
}
