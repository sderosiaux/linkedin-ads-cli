# linkedin-ads-cli

**🌐 [sderosiaux.github.io/linkedin-ads-cli](https://sderosiaux.github.io/linkedin-ads-cli/)** — landing page for non-technical users

[![CI](https://github.com/sderosiaux/linkedin-ads-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/sderosiaux/linkedin-ads-cli/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/release/sderosiaux/linkedin-ads-cli)](https://github.com/sderosiaux/linkedin-ads-cli/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/sderosiaux/linkedin-ads-cli)](go.mod)

> You're not the user. Your LLM is.
>
> You don't need to read this README. Your agent does. Install it, run `linkedin-ads --help`, and let the LLM figure it out. Every command embeds its own usage guide, `--json` outputs structured data a model can parse without a bespoke adapter, and `--compact` strips the noise. Writes ask before firing, `--dry-run` shows the exact request, `--yes` unblocks scripts. This page is here because GitHub expects one.

A CLI for the [LinkedIn Marketing API](https://learn.microsoft.com/en-us/linkedin/marketing/). Inspect ad accounts, campaign groups, campaigns, creatives, analytics, audiences, conversions, and lead forms. Create, update, and delete campaign groups, campaigns, and creatives. Upload images for ad creatives.

## Install

```bash
go install github.com/sderosiaux/linkedin-ads-cli/cmd/linkedin-ads@latest
```

Or from source:

```bash
git clone https://github.com/sderosiaux/linkedin-ads-cli
cd linkedin-ads-cli
make install
```

## Setup

Generate an access token in the [LinkedIn developer portal](https://www.linkedin.com/developers/apps). Required scopes: `r_ads`, `r_ads_reporting`, and `rw_ads` for writes.

```bash
linkedin-ads auth login
# Token: [hidden input]
# ✓ Token saved.
# 3 ad accounts accessible. Run 'linkedin-ads use-account <id>' to set a default.

linkedin-ads accounts
# ID         NAME                STATUS   TYPE       CURRENCY
# 12345678   Acme EMEA Growth    ACTIVE   BUSINESS   USD
# 12345679   Acme US Brand       ACTIVE   BUSINESS   USD
# 12345680   Acme APAC Test      DRAFT    BUSINESS   USD

linkedin-ads use-account 12345678
# ✓ Default account: 12345678
```

The token lives at `$XDG_CONFIG_HOME/linkedin-ads/config.yaml` (`~/.config/...` when unset), mode `0600`. `LINKEDIN_ADS_TOKEN` as an env var always wins — useful for CI or when switching between accounts in scripts.

LinkedIn versions its API by month. The binary ships with a recent default; bump it when LinkedIn releases a new version:

```bash
linkedin-ads config set-version 202601
# or per-call:
linkedin-ads --version-date 202601 accounts
```

## Usage

### Account snapshot

```
$ linkedin-ads overview

Account:          Acme EMEA Growth (12345678)
Currency:         USD

Campaign Groups:  4 active / 6 total
Campaigns:        12 active / 5 paused / 17 total
Spend (last 7d):  $4,238.50
Impressions (7d): 1,238,402
Clicks (7d):      8,231
```

### Reading resources

```bash
# Campaign groups and campaigns (bare form uses the default account)
linkedin-ads campaign-groups
linkedin-ads campaign-groups list --status ACTIVE
linkedin-ads campaign-groups get 678

linkedin-ads campaigns
linkedin-ads campaigns list --group 678 --status ACTIVE
linkedin-ads campaigns get 12345

# Creatives under a campaign
linkedin-ads creatives list --campaign 12345
linkedin-ads creatives get urn:li:sponsoredCreative:123456789

# Other resources
linkedin-ads audiences list
linkedin-ads conversions list
linkedin-ads leads forms list
```

### Analytics

```bash
# Rolled up by campaign for an account (defaults to last 30 days)
linkedin-ads analytics campaigns --start 2026-03-07 --end 2026-04-06

# Per-creative breakdown for a campaign
linkedin-ads analytics creatives --campaign 12345

# Demographics: JOB_FUNCTION, INDUSTRY, SENIORITY, COMPANY_SIZE, COUNTRY, REGION
# (short forms auto-map to MEMBER_JOB_FUNCTION, MEMBER_COUNTRY_V2, etc.)
linkedin-ads analytics demographics --campaign 12345 --pivot JOB_FUNCTION

# Unique reach for a campaign
linkedin-ads analytics reach --campaign 12345

# Daily timeseries (campaign-scoped or account-scoped)
linkedin-ads analytics daily-trends --campaign 12345

# Two campaigns side-by-side (spend, impressions, clicks, ctr, cpc)
linkedin-ads analytics compare --a 12345 --b 67890 --metric ctr

# Performance breakdown for conversions or lead forms
linkedin-ads conversions performance
linkedin-ads leads performance
```

### Writing

Every write prints a diff, asks before firing, and logs a correlation UUID to stderr. `--dry-run` prints the HTTP request without sending it. `--yes` skips the prompt.

```bash
# Create a campaign group
linkedin-ads campaign-groups create \
    --name "Q2 Brand Push" \
    --total-budget 5000 \
    --currency USD \
    --start 2026-04-01 \
    --end 2026-06-30

# Update (shows old → new diff, then prompts)
linkedin-ads campaign-groups update 678 --status ACTIVE

# Dry run before pausing
linkedin-ads campaigns update 12345 --status PAUSED --dry-run
# Updating campaign 12345 (Spring Promo)
#   status: ACTIVE  →  PAUSED
# POST /adCampaigns/12345
# {"patch":{"$set":{"status":"PAUSED"}}}

linkedin-ads campaigns update 12345 --status PAUSED --yes
# correlation-id: f47ac10b-58cc-4372-a567-0e02b2c3d479
# ✓ Updated.

# Delete
linkedin-ads campaigns delete 12345 --yes
```

Create a campaign:

```bash
linkedin-ads campaigns create \
    --group 678 \
    --name "Spring 2026" \
    --daily-budget 100 \
    --currency USD \
    --objective BRAND_AWARENESS \
    --type SPONSORED_UPDATES \
    --locale en_US \
    --start 2026-04-01
```

Creatives:

```bash
# Create a creative referencing an existing post
linkedin-ads creatives create --campaign 12345 --content-reference urn:li:share:789

# Create a creative with inline post content (no existing post needed)
linkedin-ads creatives create-inline \
    --campaign 12345 --org 456 --text "Check this out" \
    --image urn:li:image:ABC --landing-page https://example.com

# Change a creative's intended status
linkedin-ads creatives update-status urn:li:sponsoredCreative:123456789 --status PAUSED
```

Images:

```bash
# Upload an image for use in ad creatives
linkedin-ads images upload --file banner.png --owner 456

# Upload with media library registration
linkedin-ads images upload --file banner.png --owner 456 --account 12345678 --name "Q2 Banner"
```

## LLM and script usage

Add `--json` to any command for structured output. Add `--compact` to strip non-essential fields:

```bash
# Full account snapshot for a system prompt
linkedin-ads overview --json

# Active campaigns, minimal fields
linkedin-ads campaigns list --status ACTIVE --json --compact
```

Example `linkedin-ads campaigns list --json --compact`:

```json
[
  {
    "id": 12345,
    "name": "Spring Promo",
    "status": "ACTIVE",
    "campaignGroup": "urn:li:sponsoredCampaignGroup:678",
    "dailyBudget": {"amount": "100", "currencyCode": "USD"},
    "objectiveType": "BRAND_AWARENESS"
  }
]
```

Resolve URN references to human names (requires `--json`):

```bash
linkedin-ads campaigns list --json --resolve
# wraps output as {"data": [...], "_resolved": {"urn:li:...": "Q2 Brand Push"}}
```

Cap results:

```bash
linkedin-ads campaigns list --json --limit 5
```

Trace HTTP calls to stderr (no auth headers are ever printed):

```bash
linkedin-ads --verbose accounts list
# GET https://api.linkedin.com/rest/adAccounts?q=search (200, 142ms, 4.2KB)
```

## Global flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--compact` | Minimal fields only (requires `--json`) |
| `--limit N` | Cap array results at N items |
| `--account <id>` | Override the default account for this call |
| `--dry-run` | Print the request that would be sent without executing it |
| `--yes` | Skip confirmation prompts on writes |
| `--verbose` | Log HTTP requests to stderr (no auth headers) |
| `--version-date YYYYMM` | Override the `Linkedin-Version` header |
| `--config <path>` | Alternative config file |

`--resolve` is a per-command flag on `campaigns list` and `campaign-groups list` only.

## Status values

**Campaign groups and campaigns**

| Status | Meaning |
|--------|---------|
| `ACTIVE` | Running |
| `PAUSED` | Temporarily stopped |
| `DRAFT` | Not yet submitted |
| `ARCHIVED` | Kept for reporting, not serving |
| `COMPLETED` | Ended (hit end date or total budget) |
| `CANCELED` | Stopped before completion |
| `PENDING_DELETION` | Scheduled for deletion |
| `REMOVED` | Deleted |

**Creatives**

| Status | Meaning |
|--------|---------|
| `ACTIVE` | Serving |
| `PAUSED` | Stopped |
| `DRAFT` | Not yet submitted |
| `PENDING_REVIEW` | Under LinkedIn ad review |
| `REJECTED` | Failed review |
| `ARCHIVED` / `CANCELED` / `REMOVED` | Inactive variants |

**Accounts**: `ACTIVE`, `DRAFT`, `CANCELED`, `PENDING_DELETION`, `REMOVED`

## Configuration

| Source | Key | Priority |
|--------|-----|----------|
| `--version-date` flag | API version override | highest |
| `LINKEDIN_ADS_TOKEN` env var | token | high |
| `LINKEDIN_ADS_ACCOUNT` env var | default account | high |
| `LINKEDIN_ADS_VERSION` env var | API version | high |
| `$XDG_CONFIG_HOME/linkedin-ads/config.yaml` | token, default_account, api_version | fallback |

`$XDG_CONFIG_HOME` defaults to `~/.config` when unset. Config file written by `linkedin-ads auth login` and `linkedin-ads use-account`:

```yaml
token: your-token-here
default_account: "12345678"
api_version: "202601"
```

Permissions are enforced: the file is created `0600` and the directory `0700`. A warning is printed if the file becomes world-readable.

## Safety

Writes are explicit. Every create, update, and delete:

1. Fetches the current state (for updates) and prints a field-level diff.
2. Prompts `Continue? [y/N]` unless `--yes` is set or stdin is not a TTY.
3. Logs a `correlation-id: <uuid>` line to stderr before firing, so you can trace the call in LinkedIn's activity log.
4. `--dry-run` stops at step 1 and prints the request that would be sent.

There is no bulk delete. There is no write path for audiences, conversions, or lead forms.

## Contributing

```bash
make install-hooks   # once
make check           # tidy, vet, lint, test
```

The pre-commit hook runs `golangci-lint fmt`, `go build`, `go vet`, `golangci-lint run`, and `go mod tidy` on every commit that touches `.go` files. It aborts on any failure. Required tool: [golangci-lint](https://golangci-lint.run/) v2+.

## License

[MIT](LICENSE).
