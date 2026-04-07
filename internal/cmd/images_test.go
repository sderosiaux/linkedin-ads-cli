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

func TestImagesUpload_DryRun(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	imgPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(imgPath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--dry-run",
		"images", "upload",
		"--file", imgPath, "--owner", "789",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "initializeUpload") {
		t.Errorf("expected initializeUpload in output, got: %s", s)
	}
}

func TestImagesUpload_NameRequiresAccount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// No DefaultAccount set.
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}
	imgPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(imgPath, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--dry-run",
		"images", "upload",
		"--file", imgPath, "--owner", "789", "--name", "my-asset",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --name is set without --account")
	}
	if !strings.Contains(err.Error(), "--name requires") {
		t.Errorf("error should mention --name requires account, got: %v", err)
	}
}

func TestImagesUpload_E2E(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/images"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": map[string]any{
					"uploadUrl": "http://" + r.Host + "/upload-bin",
					"image":     "urn:li:image:ABC",
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/upload-bin":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	t.Setenv("LINKEDIN_ADS_BASE_URL", srv.URL)

	cfgPath := filepath.Join(dir, "config.yaml")
	if err := config.Save(cfgPath, &config.Config{Token: "x", APIVersion: "202601", DefaultAccount: "777"}); err != nil { //nolint:gosec // test fixture, not a real token
		t.Fatal(err)
	}

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{
		"--config", cfgPath, "--yes",
		"images", "upload",
		"--file", imgPath, "--owner", "789",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "urn:li:image:ABC") {
		t.Errorf("expected image URN, got: %s", out.String())
	}
}
