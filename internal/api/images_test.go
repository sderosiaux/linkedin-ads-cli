package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestUploadImage_TwoStepFlow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png-data"), 0o600); err != nil {
		t.Fatal(err)
	}

	var (
		initCalled bool
		putCalled  bool
		putBody    []byte
		putCT      string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/images"):
			initCalled = true
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			initReq, ok := body["initializeUploadRequest"].(map[string]any)
			if !ok {
				t.Errorf("missing initializeUploadRequest")
			}
			if initReq["owner"] != "urn:li:organization:789" {
				t.Errorf("owner: %v", initReq["owner"])
			}
			w.Header().Set("Content-Type", "application/json")
			// Use r.Host to build the upload URL — avoids referencing srv before init.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": map[string]any{
					"uploadUrl": "http://" + r.Host + "/upload-target",
					"image":     "urn:li:image:C123",
				},
			})

		case r.Method == http.MethodPut && r.URL.Path == "/upload-target":
			putCalled = true
			putCT = r.Header.Get("Content-Type")
			putBody, _ = io.ReadAll(r.Body)
			if r.Header.Get("Authorization") == "" {
				t.Error("PUT missing Authorization header")
			}
			if r.Header.Get("Linkedin-Version") != "" {
				t.Error("PUT should NOT have Linkedin-Version header")
			}
			w.WriteHeader(http.StatusCreated)

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "test-tok", APIVersion: "202601"}) //nolint:gosec // test fixture
	res, err := UploadImage(context.Background(), c, imgPath, "urn:li:organization:789", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !initCalled {
		t.Error("init not called")
	}
	if !putCalled {
		t.Error("PUT not called")
	}
	if res.ImageURN != "urn:li:image:C123" {
		t.Errorf("image URN: %q", res.ImageURN)
	}
	if putCT != "image/png" {
		t.Errorf("content-type: %q", putCT)
	}
	if string(putBody) != "fake-png-data" {
		t.Errorf("put body: %q", string(putBody))
	}
}

func TestUploadImage_WithMediaLibrary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(imgPath, []byte("jpg-data"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotInitBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&gotInitBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": map[string]any{
					"uploadUrl": "http://" + r.Host + "/put",
					"image":     "urn:li:image:XYZ",
				},
			})
		case http.MethodPut:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"}) //nolint:gosec // test fixture
	_, err := UploadImage(context.Background(), c, imgPath, "urn:li:organization:789", "555", "my-asset")
	if err != nil {
		t.Fatal(err)
	}

	initReq, ok := gotInitBody["initializeUploadRequest"].(map[string]any)
	if !ok {
		t.Fatal("missing initializeUploadRequest")
	}
	meta, ok := initReq["mediaLibraryMetadata"].(map[string]any)
	if !ok {
		t.Fatal("missing mediaLibraryMetadata")
	}
	if meta["associatedAccount"] != "urn:li:sponsoredAccount:555" {
		t.Errorf("associatedAccount: %v", meta["associatedAccount"])
	}
	if meta["assetName"] != "my-asset" {
		t.Errorf("assetName: %v", meta["assetName"])
	}
}

func TestDetectMIME(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"file.png":  "image/png",
		"file.jpg":  "image/jpeg",
		"file.jpeg": "image/jpeg",
		"file.gif":  "image/gif",
		"file.bmp":  "application/octet-stream",
	}
	for path, want := range cases {
		if got := detectMIME(path); got != want {
			t.Errorf("detectMIME(%q) = %q, want %q", path, got, want)
		}
	}
}
