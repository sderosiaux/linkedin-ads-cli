package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestConversionsList_EmptyState_Terminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[],"paging":{"start":0,"count":0,"total":0}}`))
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
	root.SetArgs([]string{"--config", cfgPath, "conversions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "No conversion definitions for account 777") {
		t.Errorf("expected empty-state hint, got: %s", s)
	}
}

func TestConversionsPerformance_JSON(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hits.Add(1)
		if r.URL.Path != "/adAnalytics" {
			t.Errorf("path: %s", r.URL.Path)
		}
		raw := r.URL.RawQuery
		if !strings.Contains(raw, "pivot=CONVERSION") {
			t.Errorf("missing pivot=CONVERSION in: %s", raw)
		}
		_, _ = w.Write([]byte(`{"elements":[
			{"pivotValue":"urn:li:lmsConversion:1","impressions":1000,"clicks":50,"externalWebsiteConversions":7,"costInUsd":"12.34"},
			{"pivotValue":"urn:li:lmsConversion:2","impressions":500,"clicks":20,"externalWebsiteConversions":3,"costInUsd":"5.00"}
		]}`))
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
	root.SetArgs([]string{"--config", cfgPath, "--json", "conversions", "performance"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if hits.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", hits.Load())
	}
	body := out.String()
	if !strings.Contains(body, `"urn:li:lmsConversion:1"`) || !strings.Contains(body, `"urn:li:lmsConversion:2"`) {
		t.Errorf("expected both conversions in output: %s", body)
	}
}
