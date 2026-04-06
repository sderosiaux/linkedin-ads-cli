# linkedin-ads CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a `linkedin-ads` Go CLI that exposes the LinkedIn Marketing API (read + limited write on campaign groups and campaigns) with LLM-friendly JSON output and kubectl-style context management.

**Architecture:** cobra + viper on top of a `net/http` client. One package per concern (`client`, `config`, `urn`, `api`, `cmd`, `format`, `resolve`, `confirm`). Fixture-driven tests via `httptest.Server`. Token + default account stored in `~/.config/linkedin-ads/config.yaml` at 0600. See `docs/plans/2026-04-06-linkedin-ads-cli-design.md` for the full design.

**Tech Stack:** Go 1.22+, `spf13/cobra` v1.9+, `spf13/viper` v1.20+, `fatih/color`, `stretchr/testify`, `goreleaser`, `golangci-lint`.

---

## Working agreements

- **TDD throughout.** Every package starts with a failing test. Scaffolding-only tasks (go.mod init, config files) are exempt.
- **Commit after each task.** Task IDs go in the commit message trailer (e.g. `Task 7`).
- **Run `go test ./...` and `golangci-lint run` before every commit.** Both must pass.
- **Never mock internal packages.** Exercise the real code paths. Mock only at the `http.RoundTripper` or `httptest.Server` boundary.
- **No work beyond the task.** If you spot something to improve outside scope, add it to `docs/plans/FOLLOWUPS.md`.
- **Use Context7 to look up cobra/viper APIs if anything is unclear.** Don't guess.

---

## Phase 1 — Project bootstrap

### Task 1: Initialize the Go module

**Files:**
- Create: `go.mod`
- Create: `cmd/linkedin-ads/main.go`
- Create: `.gitignore`

**Step 1: Init module**

```bash
cd /Users/sderosiaux/code/personal/linkedin-ads-cli
go mod init github.com/sderosiaux/linkedin-ads-cli
```

**Step 2: Write minimal main.go**

```go
// cmd/linkedin-ads/main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "linkedin-ads: not yet wired up")
	os.Exit(1)
}
```

**Step 3: Write .gitignore**

```
/linkedin-ads
/dist/
*.test
*.out
.DS_Store
```

**Step 4: Verify it builds**

Run: `go build -o linkedin-ads ./cmd/linkedin-ads`
Expected: exits 0, binary produced.

Run: `./linkedin-ads`
Expected: `linkedin-ads: not yet wired up` on stderr, exit 1.

**Step 5: Commit**

```bash
git add go.mod .gitignore cmd/linkedin-ads/main.go
git commit -m "feat: bootstrap go module (Task 1)"
```

---

### Task 2: Wire cobra root command

**Files:**
- Modify: `cmd/linkedin-ads/main.go`
- Create: `internal/cmd/root.go`
- Create: `internal/cmd/root_test.go`

**Step 1: Add cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

**Step 2: Write the failing test first**

```go
// internal/cmd/root_test.go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_ShowsHelp(t *testing.T) {
	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	if !strings.Contains(out.String(), "linkedin-ads") {
		t.Fatalf("expected name in help, got: %s", out.String())
	}
}
```

Run: `go test ./internal/cmd/...`
Expected: FAIL — package doesn't exist yet.

**Step 3: Implement `NewRootCmd`**

```go
// internal/cmd/root.go
package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "linkedin-ads",
		Short:         "LinkedIn Marketing API CLI",
		Long:          "Inspect and manage LinkedIn Ads campaigns from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
```

**Step 4: Wire it into main**

```go
// cmd/linkedin-ads/main.go
package main

import (
	"fmt"
	"os"

	"github.com/sderosiaux/linkedin-ads-cli/internal/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 5: Verify tests and build**

Run: `go test ./... && go build ./cmd/linkedin-ads`
Expected: PASS, binary builds. `./linkedin-ads --help` prints usage.

**Step 6: Commit**

```bash
git add .
git commit -m "feat: wire cobra root command (Task 2)"
```

---

### Task 3: Add golangci-lint config and CI-friendly Makefile

**Files:**
- Create: `.golangci.yml`
- Create: `Makefile`

**Step 1: Write `.golangci.yml`**

```yaml
run:
  timeout: 3m
linters:
  disable-all: true
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofumpt
    - revive
linters-settings:
  revive:
    rules:
      - name: exported
        disabled: true
```

**Step 2: Write `Makefile`**

```makefile
.PHONY: build test lint check

build:
	go build -o linkedin-ads ./cmd/linkedin-ads

test:
	go test ./... -count=1 -short

lint:
	golangci-lint run

check: test lint
```

**Step 3: Run `make check`**

Expected: tests pass, lint passes (or install golangci-lint if missing: `brew install golangci-lint`).

**Step 4: Commit**

```bash
git add .golangci.yml Makefile
git commit -m "chore: add lint config and makefile (Task 3)"
```

---

## Phase 2 — Config and URN primitives

### Task 4: Config file load / save with 0600 perms

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCreatesFileWith0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")
	c := &Config{Token: "tok", DefaultAccount: "123", APIVersion: "202601"}
	if err := Save(path, c); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %v", info.Mode().Perm())
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	orig := &Config{Token: "tok", DefaultAccount: "123", APIVersion: "202601"}
	if err := Save(path, orig); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *orig {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", got, orig)
	}
}

func TestLoadMissingFileReturnsZero(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Token != "" || got.DefaultAccount != "" {
		t.Fatalf("expected zero, got %+v", got)
	}
}
```

Run: `go test ./internal/config/...`
Expected: FAIL — package doesn't exist.

**Step 2: Implement**

```go
// internal/config/config.go
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token          string `yaml:"token"`
	DefaultAccount string `yaml:"default_account"`
	APIVersion     string `yaml:"api_version"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "linkedin-ads", "config.yaml")
}
```

Add dependency: `go get gopkg.in/yaml.v3`

**Step 3: Run tests**

Run: `go test ./internal/config/...`
Expected: PASS.

**Step 4: Commit**

```bash
git add .
git commit -m "feat(config): yaml load/save with 0600 perms (Task 4)"
```

---

### Task 5: URN wrap / unwrap helpers

**Files:**
- Create: `internal/urn/urn.go`
- Create: `internal/urn/urn_test.go`

**Step 1: Write the failing tests**

```go
// internal/urn/urn_test.go
package urn

import "testing"

func TestWrap(t *testing.T) {
	cases := []struct {
		kind Kind
		id   string
		want string
	}{
		{Account, "123", "urn:li:sponsoredAccount:123"},
		{CampaignGroup, "456", "urn:li:sponsoredCampaignGroup:456"},
		{Campaign, "789", "urn:li:sponsoredCampaign:789"},
		{Creative, "101", "urn:li:sponsoredCreative:101"},
	}
	for _, tc := range cases {
		if got := Wrap(tc.kind, tc.id); got != tc.want {
			t.Errorf("Wrap(%v, %q) = %q, want %q", tc.kind, tc.id, got, tc.want)
		}
	}
}

func TestWrapIdempotent(t *testing.T) {
	full := "urn:li:sponsoredCampaign:789"
	if got := Wrap(Campaign, full); got != full {
		t.Errorf("Wrap should be idempotent: %q", got)
	}
}

func TestUnwrap(t *testing.T) {
	cases := map[string]string{
		"urn:li:sponsoredAccount:123":       "123",
		"urn:li:sponsoredCampaignGroup:456": "456",
		"789":                               "789",
	}
	for in, want := range cases {
		if got := Unwrap(in); got != want {
			t.Errorf("Unwrap(%q) = %q, want %q", in, got, want)
		}
	}
}
```

**Step 2: Implement**

```go
// internal/urn/urn.go
package urn

import "strings"

type Kind string

const (
	Account       Kind = "sponsoredAccount"
	CampaignGroup Kind = "sponsoredCampaignGroup"
	Campaign      Kind = "sponsoredCampaign"
	Creative      Kind = "sponsoredCreative"
)

func Wrap(k Kind, id string) string {
	if strings.HasPrefix(id, "urn:li:") {
		return id
	}
	return "urn:li:" + string(k) + ":" + id
}

func Unwrap(urn string) string {
	i := strings.LastIndex(urn, ":")
	if i < 0 {
		return urn
	}
	return urn[i+1:]
}
```

**Step 3: Run tests and commit**

```bash
go test ./internal/urn/...
git add .
git commit -m "feat(urn): wrap/unwrap helpers (Task 5)"
```

---

## Phase 3 — HTTP client

### Task 6: Minimal HTTP client with auto-headers

**Files:**
- Create: `internal/client/client.go`
- Create: `internal/client/client_test.go`

**Step 1: Write the failing test**

```go
// internal/client/client_test.go
package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSetsAuthAndVersionHeaders(t *testing.T) {
	var gotAuth, gotVersion, gotRestli string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Linkedin-Version")
		gotRestli = r.Header.Get("X-Restli-Protocol-Version")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "yes"})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "tok", APIVersion: "202601"})
	var out map[string]string
	if err := c.GetJSON(context.Background(), "/ping", nil, &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth: %q", gotAuth)
	}
	if gotVersion != "202601" {
		t.Errorf("version: %q", gotVersion)
	}
	if gotRestli != "2.0.0" {
		t.Errorf("restli: %q", gotRestli)
	}
	if out["ok"] != "yes" {
		t.Errorf("body: %+v", out)
	}
}
```

Run: `go test ./internal/client/...` → FAIL.

**Step 2: Implement**

```go
// internal/client/client.go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Options struct {
	BaseURL    string
	Token      string
	APIVersion string
	HTTP       *http.Client
	Verbose    bool
}

type Client struct {
	base    string
	token   string
	version string
	http    *http.Client
	verbose bool
}

func New(o Options) *Client {
	h := o.HTTP
	if h == nil {
		h = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		base:    o.BaseURL,
		token:   o.Token,
		version: o.APIVersion,
		http:    h,
		verbose: o.Verbose,
	}
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	u, err := url.Parse(c.base + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Linkedin-Version", c.version)
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
```

**Step 3: Run tests and commit**

```bash
go test ./internal/client/...
git add .
git commit -m "feat(client): minimal HTTP client with auto headers (Task 6)"
```

---

### Task 7: Typed error parsing

**Files:**
- Create: `internal/client/errors.go`
- Create: `internal/client/errors_test.go`
- Modify: `internal/client/client.go` (use new error type)

**Step 1: Write the failing test**

```go
// internal/client/errors_test.go
package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"status":401,"code":"UNAUTHORIZED","message":"bad token","serviceErrorCode":65601}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out any
	err := c.GetJSON(context.Background(), "/foo", nil, &out)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 401 || apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("unexpected: %+v", apiErr)
	}
}
```

**Step 2: Implement `APIError` + wire into client**

```go
// internal/client/errors.go
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct {
	Status           int    `json:"status"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	ServiceErrorCode int    `json:"serviceErrorCode"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("linkedin api %d %s: %s", e.Status, e.Code, e.Message)
}

func parseError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var api APIError
	if json.Unmarshal(b, &api) == nil && api.Status != 0 {
		return &api
	}
	return fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
}
```

Then in `client.go`, replace the `fmt.Errorf(...)` branch in `GetJSON` with `return parseError(resp)`.

**Step 3: Run tests and commit**

```bash
go test ./internal/client/...
git add .
git commit -m "feat(client): typed APIError (Task 7)"
```

---

### Task 8: Retry with backoff on 429 / 5xx

**Files:**
- Create: `internal/client/retry.go`
- Create: `internal/client/retry_test.go`
- Modify: `internal/client/client.go` (wrap `do` with retry)

**Step 1: Write the failing test**

```go
// internal/client/retry_test.go
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRetryOn429ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"ok":"yes"}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var out map[string]string
	if err := c.GetJSON(context.Background(), "/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
	if out["ok"] != "yes" {
		t.Errorf("body: %+v", out)
	}
}
```

**Step 2: Implement**

```go
// internal/client/retry.go
package client

import (
	"net/http"
	"strconv"
	"time"
)

const maxAttempts = 3

func shouldRetry(status int) bool {
	return status == 429 || status == 502 || status == 503 || status == 504
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if s, err := strconv.Atoi(ra); err == nil {
				return time.Duration(s) * time.Second
			}
		}
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
```

Wrap the transport in `do()`:

```go
// inside client.go, replace the http.Do call:
for attempt := 0; ; attempt++ {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if !shouldRetry(resp.StatusCode) || attempt >= maxAttempts-1 {
		return resp, nil
	}
	d := retryDelay(resp, attempt)
	resp.Body.Close()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(d):
	}
	// rebuild body reader if needed
	if body != nil {
		b, _ := json.Marshal(body)
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
}
```

**Step 3: Run tests and commit**

```bash
go test ./internal/client/...
git add .
git commit -m "feat(client): retry on 429/5xx with backoff (Task 8)"
```

---

### Task 9: Pagination iterator for `start`/`count` style

**Files:**
- Create: `internal/client/pagination.go`
- Create: `internal/client/pagination_test.go`

**Step 1: Write the failing test**

```go
// internal/client/pagination_test.go
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginateStartCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		resp := map[string]any{
			"elements": []map[string]any{},
			"paging":   map[string]any{"start": 0, "count": 2, "total": 5},
		}
		switch start {
		case "", "0":
			resp["elements"] = []map[string]any{{"id": 1}, {"id": 2}}
		case "2":
			resp["elements"] = []map[string]any{{"id": 3}, {"id": 4}}
		case "4":
			resp["elements"] = []map[string]any{{"id": 5}}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	err := PaginateStartCount(context.Background(), c, "/items", nil, 2, 0, &all)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 items, got %d: %v", len(all), all)
	}
	for i, item := range all {
		if item["id"].(float64) != float64(i+1) {
			t.Errorf("index %d: %v", i, item)
		}
	}
	_ = fmt.Sprintf
}

func TestPaginateStartCount_HonorsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{{"id": 1}, {"id": 2}},
			"paging":   map[string]any{"start": 0, "count": 2, "total": 100},
		})
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateStartCount(context.Background(), c, "/items", nil, 2, 3, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 items (limit), got %d", len(all))
	}
}
```

**Step 2: Implement**

```go
// internal/client/pagination.go
package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type pagedResponse struct {
	Elements json.RawMessage `json:"elements"`
	Paging   struct {
		Start int `json:"start"`
		Count int `json:"count"`
		Total int `json:"total"`
	} `json:"paging"`
}

// PaginateStartCount walks a Rest.li endpoint using start/count and appends
// decoded elements into dst. If limit > 0, stops once dst has limit items.
func PaginateStartCount(ctx context.Context, c *Client, path string, q url.Values, pageSize int, limit int, dst any) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	if q == nil {
		q = url.Values{}
	}
	start := 0
	var accumulated []json.RawMessage
	for {
		q.Set("start", strconv.Itoa(start))
		q.Set("count", strconv.Itoa(pageSize))

		var page pagedResponse
		if err := c.GetJSON(ctx, path, q, &page); err != nil {
			return err
		}
		var raws []json.RawMessage
		if len(page.Elements) > 0 {
			if err := json.Unmarshal(page.Elements, &raws); err != nil {
				return err
			}
		}
		accumulated = append(accumulated, raws...)

		if limit > 0 && len(accumulated) >= limit {
			accumulated = accumulated[:limit]
			break
		}
		if len(raws) < pageSize {
			break
		}
		if page.Paging.Total > 0 && start+pageSize >= page.Paging.Total {
			break
		}
		start += pageSize
	}

	b, err := json.Marshal(accumulated)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
```

**Step 3: Run tests and commit**

```bash
go test ./internal/client/...
git add .
git commit -m "feat(client): start/count pagination iterator (Task 9)"
```

---

### Task 10: Pagination iterator for `pageToken` style

**Files:**
- Modify: `internal/client/pagination.go`
- Modify: `internal/client/pagination_test.go`

**Step 1: Add failing test**

```go
func TestPaginateToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("pageToken")
		switch token {
		case "":
			_, _ = w.Write([]byte(`{"elements":[{"id":1}],"metadata":{"nextPageToken":"abc"}}`))
		case "abc":
			_, _ = w.Write([]byte(`{"elements":[{"id":2}],"metadata":{"nextPageToken":"def"}}`))
		case "def":
			_, _ = w.Write([]byte(`{"elements":[{"id":3}]}`))
		}
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	var all []map[string]any
	if err := PaginateToken(context.Background(), c, "/x", nil, 0, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}
```

**Step 2: Implement**

```go
type pagedTokenResponse struct {
	Elements json.RawMessage `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

func PaginateToken(ctx context.Context, c *Client, path string, q url.Values, limit int, dst any) error {
	if q == nil {
		q = url.Values{}
	}
	var accumulated []json.RawMessage
	for {
		var page pagedTokenResponse
		if err := c.GetJSON(ctx, path, q, &page); err != nil {
			return err
		}
		var raws []json.RawMessage
		if len(page.Elements) > 0 {
			if err := json.Unmarshal(page.Elements, &raws); err != nil {
				return err
			}
		}
		accumulated = append(accumulated, raws...)
		if limit > 0 && len(accumulated) >= limit {
			accumulated = accumulated[:limit]
			break
		}
		if page.Metadata.NextPageToken == "" {
			break
		}
		q.Set("pageToken", page.Metadata.NextPageToken)
	}
	b, err := json.Marshal(accumulated)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
```

**Step 3: Run tests and commit**

```bash
go test ./internal/client/...
git add .
git commit -m "feat(client): pageToken pagination iterator (Task 10)"
```

---

## Phase 4 — Auth and context commands

### Task 11: `auth login` (interactive + `--token` flag)

**Files:**
- Create: `internal/cmd/auth.go`
- Create: `internal/cmd/auth_test.go`
- Modify: `internal/cmd/root.go` (register subcommand)

**Step 1: Write the failing test**

```go
// internal/cmd/auth_test.go
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

func TestAuthLoginWritesToken(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "auth", "login", "--token", "AQX_abc"})

	if err := root.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}

	c, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != "AQX_abc" {
		t.Errorf("token: %q", c.Token)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perms: %v", info.Mode().Perm())
	}
	if !strings.Contains(out.String(), "Token saved") {
		t.Errorf("output: %s", out.String())
	}
}
```

**Step 2: Add `--config` persistent flag to root**

```go
// root.go
var configPath string

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{ /* existing */ }
	root.PersistentFlags().StringVar(&configPath, "config", config.DefaultPath(), "config file")
	root.AddCommand(newAuthCmd())
	return root
}

func ConfigPath() string { return configPath }
```

**Step 3: Implement `auth login`**

```go
// internal/cmd/auth.go
package cmd

import (
	"fmt"

	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"os"
)

func newAuthCmd() *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Manage authentication"}
	auth.AddCommand(newAuthLoginCmd())
	return auth
}

func newAuthLoginCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save an API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Token: ")
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return err
				}
				token = string(b)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			c, err := config.Load(ConfigPath())
			if err != nil {
				return err
			}
			c.Token = token
			if c.APIVersion == "" {
				c.APIVersion = "202601"
			}
			if err := config.Save(ConfigPath(), c); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Token saved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "API token (skips interactive prompt)")
	return cmd
}
```

Add `golang.org/x/term` dependency: `go get golang.org/x/term`.

**Step 4: Run tests and commit**

```bash
go test ./internal/cmd/...
git add .
git commit -m "feat(cmd): auth login (Task 11)"
```

---

### Task 12: `auth logout` and `auth status`

**Files:**
- Modify: `internal/cmd/auth.go`
- Modify: `internal/cmd/auth_test.go`

**Step 1: Write tests for logout and status**

```go
func TestAuthLogoutClearsToken(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{Token: "abc"})

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	c, _ := config.Load(cfgPath)
	if c.Token != "" {
		t.Errorf("token not cleared: %q", c.Token)
	}
}

func TestAuthStatusNoToken(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	root := NewRootCmd()
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"--config", cfgPath, "auth", "status"})
	_ = root.Execute()
	if !strings.Contains(out.String(), "not authenticated") {
		t.Errorf("output: %s", out.String())
	}
}
```

**Step 2: Implement**

Add `newAuthLogoutCmd` and `newAuthStatusCmd` functions. Logout clears `c.Token` and saves. Status reads config; if token absent, prints `not authenticated`, else prints `authenticated (token ending ...XXXX)`.

**Step 3: Run tests and commit**

```bash
git commit -am "feat(cmd): auth logout and status (Task 12)"
```

---

### Task 13: `use-account` and `current-account`

**Files:**
- Create: `internal/cmd/context.go`
- Create: `internal/cmd/context_test.go`
- Modify: `internal/cmd/root.go` (register)

**Step 1: Test**

```go
func TestUseAccountPersistsDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	_ = config.Save(cfgPath, &config.Config{Token: "x"})

	root := NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "use-account", "12345"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	c, _ := config.Load(cfgPath)
	if c.DefaultAccount != "12345" {
		t.Errorf("account: %q", c.DefaultAccount)
	}
}
```

**Step 2: Implement**

```go
func newUseAccountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-account <id>",
		Short: "Set default ad account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(ConfigPath())
			if err != nil {
				return err
			}
			c.DefaultAccount = args[0]
			if err := config.Save(ConfigPath(), c); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Default account: %s\n", args[0])
			return nil
		},
	}
}
```

`current-account` reads config and prints `c.DefaultAccount` or `(none)`.

Note: account validation via API call is deferred to Task 14 once accounts API is wired.

**Step 3: Run tests and commit**

```bash
go test ./internal/cmd/...
git commit -am "feat(cmd): use-account and current-account (Task 13)"
```

---

### Task 14: `config show` and `config set-version`

**Files:** `internal/cmd/config_cmd.go`, `internal/cmd/config_cmd_test.go`

**Step 1: Test** — `config show` prints token masked, version, account. `config set-version 202601` persists.

**Step 2: Implement.**

**Step 3: Run tests and commit** with message `feat(cmd): config show and set-version (Task 14)`.

---

## Phase 5 — Client-in-cmd wiring + first read endpoint

### Task 15: Resolve `*client.Client` from config in commands

**Files:**
- Create: `internal/cmd/client.go`
- Modify: `internal/cmd/root.go` (attach helper)

**Step 1: Helper**

```go
// internal/cmd/client.go
package cmd

import (
	"errors"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/config"
)

const defaultBase = "https://api.linkedin.com/rest"

func clientFromConfig() (*client.Client, *config.Config, error) {
	c, err := config.Load(ConfigPath())
	if err != nil {
		return nil, nil, err
	}
	if c.Token == "" {
		return nil, nil, errors.New("no token — run 'linkedin-ads auth login' first")
	}
	if c.APIVersion == "" {
		c.APIVersion = "202601"
	}
	base := defaultBase
	return client.New(client.Options{
		BaseURL:    base,
		Token:      c.Token,
		APIVersion: c.APIVersion,
	}), c, nil
}
```

**Step 2: Test** (minimal — token missing returns error).

**Step 3: Commit**

```bash
git commit -am "feat(cmd): client helper reading config (Task 15)"
```

---

### Task 16: `accounts list` / `accounts get`

**Files:**
- Create: `internal/api/accounts.go`
- Create: `internal/api/accounts_test.go`
- Create: `internal/cmd/accounts.go`

**Step 1: Test the api layer with fixtures**

```go
// internal/api/accounts_test.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

func TestListAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"elements": []map[string]any{
				{"id": 12345, "name": "Acme EMEA", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
				{"id": 67890, "name": "Acme US", "status": "ACTIVE", "type": "BUSINESS", "currency": "USD"},
			},
			"paging": map[string]any{"start": 0, "count": 2, "total": 2},
		})
	}))
	defer srv.Close()

	c := client.New(client.Options{BaseURL: srv.URL, Token: "x", APIVersion: "202601"})
	accts, err := ListAccounts(context.Background(), c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 2 {
		t.Fatalf("got %d", len(accts))
	}
	if accts[0].Name != "Acme EMEA" {
		t.Errorf("name: %s", accts[0].Name)
	}
}
```

**Step 2: Implement**

```go
// internal/api/accounts.go
package api

import (
	"context"
	"net/url"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
)

type Account struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

func ListAccounts(ctx context.Context, c *client.Client, limit int) ([]Account, error) {
	q := url.Values{}
	q.Set("q", "search")
	var out []Account
	err := client.PaginateStartCount(ctx, c, "/adAccounts", q, 500, limit, &out)
	return out, err
}

func GetAccount(ctx context.Context, c *client.Client, id string) (*Account, error) {
	var a Account
	err := c.GetJSON(ctx, "/adAccounts/"+id, nil, &a)
	return &a, err
}
```

**Step 3: Add the cobra command**

```go
// internal/cmd/accounts.go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/spf13/cobra"
)

func newAccountsCmd() *cobra.Command {
	root := &cobra.Command{Use: "accounts", Short: "List and inspect ad accounts"}
	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List accessible ad accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := clientFromConfig()
			if err != nil {
				return err
			}
			accts, err := api.ListAccounts(context.Background(), c, 0)
			if err != nil {
				return err
			}
			return writeOutput(cmd, accts, func() string {
				// minimal terminal render
				s := "ID         NAME                STATUS   TYPE       CURRENCY\n"
				for _, a := range accts {
					s += fmt.Sprintf("%-10d %-19s %-8s %-10s %s\n", a.ID, a.Name, a.Status, a.Type, a.Currency)
				}
				return s
			})
		},
	})
	return root
}

// helper (can move to root.go later)
func writeOutput(cmd *cobra.Command, data any, terminalFn func() string) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), terminalFn())
	return nil
}
```

Register `newAccountsCmd()` in root. Add `--json` persistent flag if not already present.

**Step 4: Run tests and commit**

```bash
go test ./...
git commit -am "feat(cmd): accounts list and get (Task 16)"
```

---

### Task 17: Global flags `--json`, `--compact`, `--limit`

**Files:**
- Modify: `internal/cmd/root.go`
- Create: `internal/cmd/output.go`
- Create: `internal/cmd/output_test.go`

**Step 1: Move `writeOutput` into `output.go` and add compact/limit handling.**

**Step 2: Test** — using an accounts-like struct, verify `--json` produces valid JSON, `--limit 1` returns one element, `--compact` strips fields.

**Step 3: Commit** — `feat(cmd): global --json --compact --limit (Task 17)`.

---

## Phase 6 — Remaining read endpoints

Each of tasks 18-24 follows the same template: api package test (with fixture), api implementation, cmd wiring, commit. The template:

### Task template

**Files:**
- Create: `internal/api/<resource>.go`
- Create: `internal/api/<resource>_test.go`
- Create: `internal/cmd/<resource>.go`

**Step 1: Pick endpoint from design doc section 3 (Command tree).**
**Step 2: Write fixture-based test against `httptest.Server`.**
**Step 3: Implement api function.**
**Step 4: Wire cobra subcommand.**
**Step 5: Run tests; commit `feat(api): <resource> read (Task N)`.**

### Tasks

- **Task 18:** `campaign-groups list / get` — endpoint `/adCampaignGroups` with `q=search&search=(account:List(urn:li:sponsoredAccount:<id>))`.
- **Task 19:** `campaigns list / get` — endpoint `/adCampaigns`, supports `--group` filter via `campaignGroup` search criterion.
- **Task 20:** `creatives list / get` — endpoint `/adCreatives`, requires `--campaign`.
- **Task 21:** `analytics campaigns` — endpoint `/adAnalytics?q=statistics` with `pivot=CAMPAIGN`, `dateRange`, `timeGranularity`. Dates encoded as `(start:(year:Y,month:M,day:D),end:(year:...))`.
- **Task 22:** `analytics creatives / demographics / reach / daily-trends / compare` — all variants of the `adAnalytics` finder with different `pivot` and `timeGranularity`. `compare` is client-side: fetch two campaigns and diff locally.
- **Task 23:** `audiences list` — `/dmpSegments?q=account&account=<urn>`.
- **Task 24:** `conversions list / performance` — `/conversions?q=account&account=<urn>`.
- **Task 25:** `leads forms / performance` — `/leadGenForms?q=account` and `/leadAnalytics`.

**Commit pattern:** one commit per task. Message `feat(api): <resource> (Task N)`.

**After task 25 run the full test suite:**

```bash
make check
```

All tests and lint must pass before moving on.

---

### Task 26: `overview` command (account health summary)

**Files:**
- Create: `internal/cmd/overview.go`
- Create: `internal/cmd/overview_test.go`

**Behavior:** fetch in parallel the number of active campaigns, paused campaigns, total spend last 7 days, number of campaign groups, recent violations (if any). Render a compact summary table. `--json` emits the struct.

**Step 1: Test** using a `httptest.Server` that dispatches on path.

**Step 2: Implement** using `errgroup.Group` for parallel fetches.

**Step 3: Commit** — `feat(cmd): overview summary (Task 26)`.

---

### Task 27: `--resolve` flag + resolver cache

**Files:**
- Create: `internal/resolve/resolver.go`
- Create: `internal/resolve/resolver_test.go`
- Modify: `internal/cmd/output.go` to call resolver when `--resolve` set.

**Step 1: Test** — the resolver given a list of campaign URNs does one batched call to `/adCampaigns?ids=List(...)` and caches the result with 5-minute TTL.

**Step 2: Implement** using a `sync.RWMutex`-protected `map[string]cacheEntry`.

**Step 3: Commit** — `feat(resolve): URN→name cache with 5min TTL (Task 27)`.

---

## Phase 7 — Write operations

### Task 28: `confirm` package

**Files:**
- Create: `internal/confirm/confirm.go`
- Create: `internal/confirm/confirm_test.go`

**Behavior:** `Prompt(in io.Reader, out io.Writer, msg string) (bool, error)` — reads a line, returns true only if it starts with `y`/`Y`. Respects TTY detection via `isatty`.

**Step 1: Tests with `bytes.Buffer`.**
**Step 2: Implement.**
**Step 3: Commit** — `feat(confirm): Y/N prompt (Task 28)`.

---

### Task 29: `campaign-groups create`

**Files:**
- Modify: `internal/api/campaign_groups.go`
- Modify: `internal/cmd/campaign_groups.go`
- Modify: `internal/api/campaign_groups_test.go`

**Step 1: Test** `CreateCampaignGroup` against a `httptest.Server` that expects POST `/adCampaignGroups` with a specific body shape and responds `201 Created` with the new URN in the `Location` or `X-LinkedIn-Id` header.

**Step 2: Implement.** Body shape per LinkedIn spec:

```json
{
  "account": "urn:li:sponsoredAccount:<id>",
  "name": "<name>",
  "status": "DRAFT",
  "totalBudget": {"currencyCode": "USD", "amount": "5000"},
  "runSchedule": {"start": 1745000000000, "end": 1750000000000}
}
```

**Step 3: Wire the cobra command** with flags `--name`, `--total-budget`, `--currency`, `--start`, `--end`. Before POST:
1. Build the payload.
2. If `--dry-run`, print `POST /adCampaignGroups\n<pretty JSON>` and return.
3. Call `confirm.Prompt` unless `--yes` or non-TTY+`--yes`.
4. POST.
5. Print the new ID.

**Step 4: Run tests and commit** — `feat(api): campaign-groups create (Task 29)`.

---

### Task 30: `campaign-groups update` / `delete`

Same template: tests first, implement `PATCH /adCampaignGroups/<id>` with Rest.li partial update body `{"patch":{"$set":{...}}}`, and `DELETE /adCampaignGroups/<id>`.

Commit: `feat(api): campaign-groups update/delete (Task 30)`.

---

### Task 31: `campaigns create / update / delete`

Same template as tasks 29 and 30. Three commands, one commit per subcommand, or a single task with three sub-steps if they are parallel.

Commit: `feat(api): campaigns write ops (Task 31)`.

---

### Task 32: `--dry-run` e2e test

**Files:** `internal/cmd/dryrun_test.go`

**Step 1: Test** — invoke `campaign-groups create --dry-run --name x ...` and assert:
- No HTTP call made (use a server whose handler calls `t.Fatal`).
- Output contains `POST /adCampaignGroups` and the JSON body.
- Exit code 0.

**Step 2: Commit** — `test(cmd): dry-run e2e coverage (Task 32)`.

---

## Phase 8 — Packaging and docs

### Task 33: `goreleaser.yaml`

**Files:**
- Create: `.goreleaser.yaml`

**Step 1: Config** — build for `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`. Homebrew tap at `sderosiaux/homebrew-tap`. Archives as tar.gz with checksums.

**Step 2: Validate** locally: `goreleaser check`.

**Step 3: Commit** — `build: add goreleaser config (Task 33)`.

---

### Task 34: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yaml`

**Step 1: Single workflow** running on push + PR:

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go test ./... -race
      - uses: golangci/golangci-lint-action@v4
        with: { version: v1.60 }
      - uses: goreleaser/goreleaser-action@v6
        with: { args: check, version: latest }
```

**Step 2: Commit** — `ci: add github actions workflow (Task 34)`.

---

### Task 35: README

**Files:**
- Create: `README.md`

**Step 1: Draft** following segment-cli README structure: install, setup (`auth login` → `use-account`), command reference, global flags, LLM usage patterns, architecture.

**Step 2: Invoke the humanizer skill** to remove AI tropes before committing.

**Step 3: Commit** — `docs: add README (Task 35)`.

---

## Verification after the full plan

```bash
make check
go build ./cmd/linkedin-ads
./linkedin-ads --help
./linkedin-ads auth login --token $LINKEDIN_ADS_TOKEN
./linkedin-ads accounts --json | jq '.[].id'
./linkedin-ads overview
```

All commands should return within a few seconds against a real LinkedIn ad account. Lint and tests must be green.

---

## Follow-ups (NOT in this plan)

- Shell completions (`completion bash|zsh|fish`)
- Image/creative uploads
- Bulk import from CSV
- Live webhook tap equivalent (LinkedIn does not expose this today)
- Multi-profile support
