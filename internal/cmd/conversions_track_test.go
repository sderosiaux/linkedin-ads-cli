package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestHashSHA256Email(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"user@example.com", "b4c9a289323b21a01c3e940f150eb9b8c542587f1abfd8f0e1cc1ffc5e475514"},
		{"  USER@Example.COM ", "b4c9a289323b21a01c3e940f150eb9b8c542587f1abfd8f0e1cc1ffc5e475514"},
	}
	for _, c := range cases {
		got := hashSHA256Email(c.in)
		if len(got) != 64 {
			t.Errorf("expected 64-char hex, got %d", len(got))
		}
		if got != c.want {
			t.Errorf("hashSHA256Email(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestConversionsTrack_DryRunBuildsExpectedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry-run should not call HTTP, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--dry-run",
		"conversions", "track", "26343066",
		"--email", "User@Example.com",
		"--value", "50000", "--currency", "USD",
		"--occurred-at", "2026-04-01",
		"--event-id", "deal-1",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	s := out.String()
	for _, want := range []string{
		"POST /conversionEvents",
		`"conversion": "urn:lla:llaPartnerConversion:26343066"`,
		`"idType": "SHA256_EMAIL"`,
		`"eventId": "deal-1"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in dry-run output:\n%s", want, s)
		}
	}
	// Hash must be 64 hex chars and match the lowercase canonical form.
	if !strings.Contains(s, hashSHA256Email("user@example.com")) {
		t.Errorf("expected canonical hash in body, got: %s", s)
	}
}

func TestConversionsTrack_PostsToConversionEvents(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"conversions", "track", "999",
		"--email", "alice@example.com",
		"--occurred-at", "2026-04-01",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if gotPath != "/conversionEvents" {
		t.Errorf("path: %q", gotPath)
	}
	if gotBody["conversion"] != "urn:lla:llaPartnerConversion:999" {
		t.Errorf("conversion urn wrong: %v", gotBody["conversion"])
	}
	// epoch millis = 2026-04-01 UTC
	if v, ok := gotBody["conversionHappenedAt"].(float64); !ok || v <= 0 {
		t.Errorf("conversionHappenedAt missing or wrong type: %v", gotBody["conversionHappenedAt"])
	}
	user, ok := gotBody["user"].(map[string]any)
	if !ok {
		t.Fatalf("user shape: %v", gotBody["user"])
	}
	ids, ok := user["userIds"].([]any)
	if !ok || len(ids) != 1 {
		t.Fatalf("userIds: %v", user["userIds"])
	}
	first := ids[0].(map[string]any)
	if first["idType"] != "SHA256_EMAIL" {
		t.Errorf("idType: %v", first["idType"])
	}
	hashStr, _ := first["idValue"].(string)
	if len(hashStr) != 64 {
		t.Errorf("idValue should be 64 hex chars, got %d", len(hashStr))
	}
}

func TestConversionsTrackBatch_CSV(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "events.csv")
	csvData := "email,occurred_at,value,currency,event_id\n" +
		"alice@example.com,2026-04-01,100,USD,e1\n" +
		"bob@example.com,2026-04-02,200,USD,e2\n"
	if err := os.WriteFile(csvPath, []byte(csvData), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"conversions", "track-batch", "999",
		"--file", csvPath,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, out.String())
	}
	if calls != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", calls)
	}
	if !strings.Contains(out.String(), "Sent 2") {
		t.Errorf("expected 'Sent 2' in output: %s", out.String())
	}
}
