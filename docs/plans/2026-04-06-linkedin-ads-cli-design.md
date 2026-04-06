# linkedin-ads CLI — Design

**Date:** 2026-04-06
**Status:** Approved, ready for implementation plan

## Goal

A single-binary CLI for LinkedIn Marketing API: inspect ad accounts, campaign groups, campaigns, creatives, analytics, audiences, conversions, and leads. Limited write operations on campaign groups and campaigns. Optimized for humans at a terminal and for LLM agents piping JSON.

Inspired by the endpoint surface of [linkedin-ads-mcp](https://github.com/danielpopamd/linkedin-ads-mcp). Architecture inspired by `segment-cli`, ported to Go.

## Decisions

| Decision | Choice |
|---|---|
| Scope | Read + limited write (campaign groups, campaigns) |
| Write safety | Interactive confirm + `--dry-run` + `--yes` for scripts |
| Binary name | `linkedin-ads` |
| Language | Go |
| CLI framework | `spf13/cobra` + `spf13/viper` |
| Distribution | `go install` + `goreleaser` (brew tap, multi-OS binaries) |
| Token storage | YAML file at `~/.config/linkedin-ads/config.yaml`, chmod 0600 |
| Context management | kubectl-style — `auth login`, `use-account`, `current-account` |

## Non-goals

- OAuth flow. Token is pasted by the user after generating it manually in the LinkedIn developer portal.
- Multi-profile (e.g. staging vs prod). Single config, single current account.
- Write ops on creatives, audiences, conversions, leads. Out of scope for v1.
- Interactive TUI. `--json` is the primary LLM path; terminal output stays lean.

## Architecture

```
linkedin-ads-cli/
├── cmd/linkedin-ads/main.go        # entry, cobra Execute
├── internal/
│   ├── client/
│   │   ├── client.go               # HTTP, Bearer auth, retry, headers
│   │   ├── pagination.go           # start/count + link-rel=next + pageToken
│   │   └── errors.go               # LinkedIn error shape → Go error
│   ├── config/
│   │   ├── config.go               # viper load/save YAML, 0600
│   │   └── state.go                # Token, DefaultAccount, APIVersion
│   ├── urn/urn.go                  # "123" ↔ "urn:li:sponsoredCampaign:123"
│   ├── api/                        # one file per resource
│   │   ├── accounts.go
│   │   ├── campaign_groups.go
│   │   ├── campaigns.go
│   │   ├── creatives.go
│   │   ├── analytics.go
│   │   ├── audiences.go
│   │   ├── demographics.go
│   │   ├── conversions.go
│   │   └── leads.go
│   ├── cmd/                        # cobra subcommands
│   │   ├── root.go                 # persistent flags, OnInitialize
│   │   ├── auth.go                 # login / logout / status
│   │   ├── context.go              # use-account / current-account
│   │   ├── config.go               # set-version / show
│   │   ├── accounts.go
│   │   ├── campaign_groups.go
│   │   ├── campaigns.go
│   │   ├── creatives.go
│   │   ├── analytics.go
│   │   ├── audiences.go
│   │   ├── conversions.go
│   │   ├── leads.go
│   │   └── overview.go
│   ├── format/                     # terminal renderers (fatih/color)
│   ├── resolve/resolver.go         # in-memory URN→name cache, 5min TTL
│   └── confirm/confirm.go          # Y/N prompts for writes
├── testdata/                       # fixture responses
├── .goreleaser.yaml
├── go.mod
└── README.md
```

Layout follows the standard Go project convention: `cmd/<binary>` for entry, `internal/` for everything else so nothing leaks as a public package.

## Config & state

**File:** `~/.config/linkedin-ads/config.yaml` — directory 0700, file 0600.

```yaml
token: AQX...
default_account: "123456789"
api_version: "202601"
```

**Resolution order** (viper): flag > env (`LINKEDIN_ADS_TOKEN`, `LINKEDIN_ADS_ACCOUNT`, `LINKEDIN_ADS_VERSION`) > file.

No manual editing required. All fields are managed by commands.

### Bootstrap flow

```
$ linkedin-ads auth login
Token: [hidden input]
✓ Token saved. 3 ad accounts accessible.
  Run 'linkedin-ads use-account <id>' to set a default.

$ linkedin-ads accounts
ID         NAME                STATUS   TYPE       CURRENCY
12345678   Acme EMEA Growth    ACTIVE   BUSINESS   USD
12345679   Acme US Brand       ACTIVE   BUSINESS   USD
12345680   Acme APAC Test      DRAFT    BUSINESS   USD

$ linkedin-ads use-account 12345678
✓ Default account: Acme EMEA Growth (12345678)

$ linkedin-ads campaigns
(uses default account)
```

### Management commands

```
linkedin-ads auth login [--token <tok>]
linkedin-ads auth logout
linkedin-ads auth status
linkedin-ads use-account <id>
linkedin-ads current-account
linkedin-ads config set-version <YYYYMM>
linkedin-ads config show                  # token masked as ***
```

## Command tree

### Read

```
linkedin-ads overview                         Account health summary

linkedin-ads accounts [list]
linkedin-ads accounts get <id>

linkedin-ads campaign-groups [list]           [--status ACTIVE|PAUSED|DRAFT|...]
linkedin-ads campaign-groups get <id>

linkedin-ads campaigns [list]                 [--group <id>] [--status ...]
linkedin-ads campaigns get <id>

linkedin-ads creatives [list] --campaign <id>
linkedin-ads creatives get <id>

linkedin-ads analytics campaigns              --start <date> [--end <date>] [--granularity DAILY|MONTHLY]
linkedin-ads analytics creatives              --campaign <id>
linkedin-ads analytics demographics           --campaign <id> [--pivot JOB_FUNCTION|INDUSTRY|SENIORITY|COMPANY_SIZE|COUNTRY|REGION]
linkedin-ads analytics reach                  --campaign <id>
linkedin-ads analytics daily-trends           [--campaign <id>]
linkedin-ads analytics compare                --a <id> --b <id> [--metric spend|impressions|ctr|cpc|conversions]

linkedin-ads audiences [list]

linkedin-ads conversions [list]
linkedin-ads conversions performance

linkedin-ads leads forms [list]
linkedin-ads leads performance                [--form <id>]
```

### Write

```
linkedin-ads campaign-groups create --name X --total-budget N --currency USD [--start ...] [--end ...]
linkedin-ads campaign-groups update <id> [--status ACTIVE|PAUSED] [--name ...] [--end ...]
linkedin-ads campaign-groups delete <id>

linkedin-ads campaigns create --group <id> --name X --daily-budget N --objective ... --type ... --locale en_US
linkedin-ads campaigns update <id> [--status ...] [--daily-budget ...] [--bid ...]
linkedin-ads campaigns delete <id>
```

Every write:

1. Prints a summary of the change (diff against current state for updates).
2. Prompts `Continue? [y/N]` unless `--yes` is set or stdin is not a TTY.
3. Executes and returns the new state.
4. `--dry-run` stops at step 1 and prints the HTTP request that would be sent.

## Global flags (cobra `PersistentFlags`)

| Flag | Role |
|---|---|
| `--json` | JSON output. LLM/scripts path. |
| `--compact` | Whitelist essential fields. Requires `--json`. |
| `--limit N` | Cap array results at N. Stops pagination early. |
| `--resolve` | Enrich URNs with human names via the resolver cache. |
| `--account <id>` | Override default account for this call. |
| `--dry-run` | No write. Print the request that would be sent. |
| `--yes` | Skip confirmation prompts. |
| `--version-date YYYYMM` | Override the `Linkedin-Version` header. |
| `--config <path>` | Alternative config file. |
| `-v, --verbose` | Log HTTP requests to stderr. |

## HTTP client (`internal/client`)

**Base:** `https://api.linkedin.com/rest`

**Headers set on every request:**

```
Authorization: Bearer <token>
Linkedin-Version: <config.api_version>
X-Restli-Protocol-Version: 2.0.0
Accept: application/json
Content-Type: application/json     (writes only)
```

**Retry:** 3 attempts max with exponential backoff on 429 / 502 / 503 / 504. Honors `Retry-After` when present. Never retries 4xx other than 429.

### Pagination

This is the single most error-prone part of the client because LinkedIn Marketing API mixes two pagination styles across endpoints.

**Style 1: Rest.li `start` + `count` (majority of endpoints).**
Response envelope:
```json
{
  "elements": [...],
  "paging": {
    "start": 0,
    "count": 500,
    "total": 1234,
    "links": [{"rel": "next", "href": "https://api.linkedin.com/rest/..."}]
  }
}
```
Client behavior:
- Default `count=500`. Max supported is 1000 but 500 is recommended to avoid payload issues.
- Follow `paging.links[rel=next]` when present. When absent, stop when `elements` has fewer items than `count`, OR when `start + count >= paging.total` if `total` is provided.
- `--limit N` stops the iterator as soon as N items have been accumulated.

**Style 2: `pageToken` (newer finders, some Ad Analytics endpoints).**
Response envelope includes `metadata.nextPageToken` or an opaque `pageToken` in `paging`. Client detects and feeds it back as a query param on the next call.

**Implementation.** A single `Iterator[T]` helper in `pagination.go` handles both styles:

```go
type Iterator[T any] struct {
    client   *Client
    req      *http.Request
    limit    int
    // internal
    collected []T
    nextHref  string
    nextToken string
}

func (it *Iterator[T]) All(ctx context.Context) ([]T, error) { ... }
```

The API layer picks the right style per endpoint via a strategy flag when constructing the iterator.

**Concurrency:** pagination is sequential. No concurrent page fetches in v1. If a command proves slow we revisit.

### URN handling

Users pass bare IDs (`12345`) wherever possible. The `urn` package wraps into full URNs at the client boundary based on the resource context:

```
accounts    → urn:li:sponsoredAccount:<id>
camp-groups → urn:li:sponsoredCampaignGroup:<id>
campaigns   → urn:li:sponsoredCampaign:<id>
creatives   → urn:li:sponsoredCreative:<id>
```

Output shows bare IDs in terminal format and full URNs in `--json` (matches what the API returns).

URL-encoding of URNs in path and query is handled by the client — never by callers.

### Errors

LinkedIn errors follow a consistent shape:

```json
{"status":401,"code":"UNAUTHORIZED","message":"...","errorDetailType":"...","serviceErrorCode":65601}
```

Parsed into a typed `APIError` and rendered with color on stderr. Exit code non-zero. A few cases are special-cased with actionable hints:

| Status | Hint |
|---|---|
| 401 | `Run 'linkedin-ads auth login' to refresh your token.` |
| 403 | Prints `serviceErrorCode` and (if returned) the missing scope. |
| 429 | Already retried automatically. Final failure shows `Retry-After` seconds. |
| `ENOTOKEN` (pre-flight) | `No token. Run 'linkedin-ads auth login' first.` |
| `ENOACCOUNT` (pre-flight) | `No default account. Run 'linkedin-ads use-account <id>' or pass --account.` |

## Output

**`--json`** — the full unwrapped payload. The client strips Rest.li envelopes (`elements`, `paging`) and returns a bare array or object. Field names are kept as the API returns them (camelCase).

**`--compact`** — per-resource whitelists. Rough set:

| Resource | Fields |
|---|---|
| account | id, name, status, type, currency |
| campaign-group | id, name, status, totalBudget, runSchedule |
| campaign | id, name, status, campaignGroup, dailyBudget, objectiveType |
| creative | id, status, intendedStatus, campaign, review |
| analytics row | dateRange, impressions, clicks, costInUsd, conversions |

**`--resolve`** — after the main fetch, the resolver issues parallel lookups (`GET /adCampaignGroups/<id>`, etc.) and attaches a `_resolved` field per object. Cached in memory for 5 minutes to keep pipelines cheap.

**Terminal (no `--json`).** Compact aligned columns via `fatih/color`. No tables from `lipgloss` — unnecessary weight for this use case. Empty states are actionable:

```
$ linkedin-ads campaigns
No campaigns in account 12345678.
Create one with: linkedin-ads campaigns create --group <id> --name ... --daily-budget ...
```

## Testing

- `internal/client`: fixtures replayed via `httptest.Server`. Covers retry, pagination (both styles), error parsing, header injection.
- `internal/urn`: unit tests for roundtrip.
- `internal/api/*`: fixture-based tests, one per endpoint, JSON stored in `testdata/`.
- Commands: cobra commands invoked via `rootCmd.SetArgs()` with stdout/stderr captured. Asserts on both human and `--json` outputs.
- Lint: `golangci-lint run` with `errcheck`, `govet`, `staticcheck`, `gofumpt`, `revive`.
- CI: single GitHub Actions workflow — `go test ./...`, `golangci-lint`, `goreleaser check`.

## Security notes

- Config file is written with `0600` and the parent directory with `0700`. A startup check warns if permissions are looser.
- `linkedin-ads config show` masks the token as `***`.
- `--verbose` never logs the `Authorization` header. Requests are logged as `METHOD url (status, duration, bytes)`.
- Write operations log a UUID correlation ID to stderr so the change is traceable in the LinkedIn activity log.

## Out of scope for v1 (explicit)

- Bulk create / import from CSV.
- Creative uploads (image / video). The MCP has `upload_image` and `create_inline_ad`; v1 only reads creatives.
- Lead form creation or lead export.
- Scheduled / recurring jobs.
- Webhook tap (segment-cli has `sources tap`; LinkedIn does not expose the equivalent).
- Shell completions. Added in v1.1 once the command tree stabilizes.

## Open items to decide during implementation

1. Exact pivot names for demographics (LinkedIn uses internal constants — confirm against the `adAnalytics` finder docs at build time).
2. Whether to fetch `adAnalytics` via the `analytics` finder or the newer `statistics` finder. Benchmark both on a real account before locking in.
3. Default `api_version` value at build time. Will be pinned to the latest stable version as of release.

## References

- LinkedIn Marketing API docs — `https://learn.microsoft.com/en-us/linkedin/marketing/`
- `segment-cli` — sibling project, source of the architecture
- `linkedin-ads-mcp` — source of the endpoint taxonomy
- `spf13/cobra` / `spf13/viper` — CLI framework
