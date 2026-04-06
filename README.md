# linkedin-ads

Read-only and limited write access to the [LinkedIn Marketing API](https://learn.microsoft.com/en-us/linkedin/marketing/). Inspect ad accounts, campaigns, creatives, analytics, audiences, conversions, and lead forms from the terminal.

Built for governance, observability, and LLM-actionable output.

## Install

```bash
go install github.com/sderosiaux/linkedin-ads-cli/cmd/linkedin-ads@latest
```

Homebrew tap is coming once the first tag ships.

Build from source:

```bash
git clone https://github.com/sderosiaux/linkedin-ads-cli.git
cd linkedin-ads-cli
make build
./linkedin-ads --help
```

## Setup

1. Generate an access token at the [LinkedIn developer portal](https://www.linkedin.com/developers/apps). The token needs the `r_ads`, `r_ads_reporting`, and (for write operations) `rw_ads` scopes.

2. Save it:

```bash
linkedin-ads auth login
# paste token at the prompt, or:
linkedin-ads auth login --token <token>
```

3. Pick an account:

```bash
linkedin-ads accounts list
linkedin-ads use-account 123456789
linkedin-ads current-account
```

4. Optional: bump the LinkedIn-Version header (defaults to a recent value baked into the binary):

```bash
linkedin-ads config set-version 202601
```

The token and config live at `~/.config/linkedin-ads/config.yaml` with mode `0600`.

## Usage

```
linkedin-ads overview                            One-screen account snapshot
linkedin-ads accounts list                       List accessible ad accounts
linkedin-ads accounts get <id>                   Single account detail
linkedin-ads use-account <id>                    Set the default account
linkedin-ads current-account                     Print the default account
linkedin-ads auth login [--token T]              Save an API token
linkedin-ads auth status                         Show auth state
linkedin-ads auth logout                         Clear the token
linkedin-ads config show                         Print config (token masked)
linkedin-ads config set-version <YYYYMM>         Set LinkedIn-Version header
linkedin-ads campaign-groups list                List groups under an account
linkedin-ads campaign-groups get <id>            Single group detail
linkedin-ads campaign-groups create ...          Create a group
linkedin-ads campaign-groups update <id> ...     Patch a group
linkedin-ads campaign-groups delete <id>         Delete a group
linkedin-ads campaigns list                      List campaigns
linkedin-ads campaigns get <id>                  Single campaign detail
linkedin-ads campaigns create ...                Create a campaign
linkedin-ads campaigns update <id> ...           Patch a campaign
linkedin-ads campaigns delete <id>               Delete a campaign
linkedin-ads creatives list --campaign <id>      List creatives under a campaign
linkedin-ads creatives get <urn>                 Single creative detail
linkedin-ads analytics campaigns                 Analytics rolled up by campaign
linkedin-ads analytics creatives --campaign <id> Analytics rolled up by creative
linkedin-ads analytics demographics --campaign <id> --pivot <P>  Demographic breakdown
linkedin-ads analytics reach --campaign <id>     Approximate unique reach
linkedin-ads analytics daily-trends              Daily timeseries
linkedin-ads analytics compare --a <id> --b <id> Compare two campaigns
linkedin-ads audiences list                      List DMP segments
linkedin-ads conversions list                    List conversion definitions
linkedin-ads leads forms list                    List lead-gen forms
```

Every read command supports `--json` for structured output. Every write command supports `--dry-run` and `--yes`.

## Global flags

| Flag | Purpose |
|---|---|
| `--json` | Structured JSON output for LLM/script consumption |
| `--compact` | Minimal JSON fields. Pairs with `--json`. |
| `--limit N` | Cap array results at N items |
| `--resolve` | Enrich URN references with human names (cached per run) |
| `--dry-run` | Print the request that would be sent without executing it |
| `--yes` | Skip confirmation prompts on writes |
| `--config <path>` | Override config file path (default `~/.config/linkedin-ads/config.yaml`) |

Account-scoped commands also accept `--account <id>` to override the default for a single call.

## Examples

Workspace snapshot:

```bash
linkedin-ads overview
linkedin-ads overview --json
```

List active campaigns as compact JSON:

```bash
linkedin-ads campaigns list --status ACTIVE --json --compact
```

Pipe through `jq` for filtering:

```bash
linkedin-ads campaigns list --json --compact | jq '[.[] | select(.status=="ACTIVE")]'
```

Last 30 days of spend by campaign:

```bash
linkedin-ads analytics campaigns --start 2026-03-07 --end 2026-04-06 --json
```

Daily trend for one campaign:

```bash
linkedin-ads analytics daily-trends --campaign 12345 --start 2026-03-30 --json
```

Demographic breakdown by job function:

```bash
linkedin-ads analytics demographics --campaign 12345 --pivot JOB_FUNCTION --json
```

Pause a campaign with a dry run first:

```bash
linkedin-ads campaigns update 12345 --status PAUSED --dry-run
linkedin-ads campaigns update 12345 --status PAUSED --yes
```

Resolve URNs to human names in any read command:

```bash
linkedin-ads campaigns list --resolve --json
```

## Architecture

```
cmd/linkedin-ads/        Binary entrypoint (version vars, root cmd wiring)
internal/
  api/                   Typed wrappers around the LinkedIn Marketing API
  client/                HTTP client, retries, pagination, write methods
  cmd/                   cobra commands (one file per resource group)
  config/                YAML load/save at 0600
  confirm/               Y/N prompt and dry-run plumbing
  resolve/               URN -> name cache for --resolve
  urn/                   URN wrap/unwrap helpers
```

## Design

- Read access plus a small surface of writes (create, update, delete on campaigns and groups). Reads cover analytics, audiences, conversions, lead forms, creatives, and accounts.
- JSON on every command for LLM and script consumers. `--compact` strips fields down to ids and labels.
- Auto-pagination on list endpoints. Both `start/count` and `pageToken` styles are handled by the client.
- Retry with exponential backoff on 429 and 5xx.
- Token stored at `~/.config/linkedin-ads/config.yaml` with mode `0600`. The `config show` command masks it.
- Writes prompt for confirmation. `--dry-run` prints the request without sending it. `--yes` skips the prompt.
- URN resolution is opt-in via `--resolve` and cached for the duration of a single command.

## Stack

Go 1.22, [cobra](https://github.com/spf13/cobra), `gopkg.in/yaml.v3`, `golang.org/x/term`. Built and released with [goreleaser](https://goreleaser.com/), linted with [golangci-lint](https://golangci-lint.run/) v2.

## License

[MIT](LICENSE).
