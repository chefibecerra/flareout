# FlareOut

FlareOut is an open-source CLI/TUI for managing Cloudflare DNS proxy state. It was built after the November 2025 Cloudflare outage, where the proxy layer broke and many sites became unreachable until operators could individually unproxy their records through a slow web dashboard.

The tool gives you three things you do not get from `dash.cloudflare.com`:

- **A multi-zone view in one screen.** See every record across every zone your token can read, marked `[P]` for proxied or `[-]` for not.
- **An interactive toggle flow.** Mark records with space, review the diff, confirm with `a`, apply.
- **A safety stack.** Every write produces a JSON snapshot of the pre-change record and a JSONL audit entry. `flareout undo` reverses the latest applied change. `flareout panic` un-proxies an entire zone in one operation when seconds matter.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/chefibecerra/flareout/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/chefibecerra/flareout/actions/workflows/ci.yml)

## Install

### Quick install (Linux / macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/chefibecerra/flareout/main/scripts/install.sh | sh
```

By default the binary lands in `~/.local/bin/flareout`. Override the prefix with `PREFIX=/usr/local sudo sh install.sh` or download the archive manually from the [releases page](https://github.com/chefibecerra/flareout/releases) and put `flareout` on your `PATH`.

### Go

```sh
go install github.com/chefibecerra/flareout/cmd/flareout@latest
```

### From source

```sh
git clone https://github.com/chefibecerra/flareout
cd flareout
go build -o flareout ./cmd/flareout
```

## Quick start

1. Create a Cloudflare API token at [dash.cloudflare.com/profile/api-tokens](https://dash.cloudflare.com/profile/api-tokens). For read operations use **Zone:Read + DNS:Read**. For writes use **Zone:Edit + DNS:Edit** scoped to the zones you want to manage.

2. Export it:

   ```sh
   export CLOUDFLARE_API_TOKEN=your-token-here
   ```

3. List your records:

   ```sh
   flareout list
   ```

   You get an interactive table. Press `space` to mark records, `a` to review and apply, `q` to quit without changing anything.

## Commands

### `flareout list`

Shows every DNS record across every zone the token can read. By default launches an interactive TUI; pipes to JSON when stdout is not a TTY, or use `--json` to force JSON regardless.

```sh
flareout list                       # TUI (default on a real terminal)
flareout list --json | jq '.[]'    # JSON for scripts
flareout list | grep proxied        # auto-detected JSON when piped
```

In the TUI:

| Key             | Action                                                    |
|-----------------|-----------------------------------------------------------|
| `space`         | Mark the record under the cursor as a pending toggle      |
| `space` again   | Unmark it                                                 |
| `a`             | Quit the TUI, review pending toggles, confirm and apply   |
| `q` / `Esc`     | Quit without applying anything                            |
| arrow keys, `j`/`k` | Navigate                                              |

### `flareout toggle <zone>/<name>`

Toggle a single record from the command line. Default mode is dry-run — pass `--apply` to actually write.

```sh
flareout toggle example.com/www --proxied=false             # dry-run, prints the diff
flareout toggle example.com/www --proxied=false --apply     # apply for real
flareout toggle example.com/www --type=A --proxied=true     # disambiguate when name has multiple types
```

### `flareout undo`

Read the most recent applied entry from the audit log, recover the pre-mutation record from its snapshot, and revert the proxied flag. The undo is itself recorded with a new snapshot and audit entry — running `undo` twice in a row redoes the original change.

```sh
flareout undo
```

### `flareout panic --zone <name>`

Un-proxy every currently-proxied record in a zone, in one operation. Built for "Cloudflare's proxy is down right now and I need traffic off of it for ALL my records under <zone>". Shows a per-record preview, then requires you to type the zone name verbatim before any write happens. Use `--yes` to skip the prompt in scripts.

```sh
flareout panic --zone example.com           # interactive, type zone to confirm
flareout panic --zone example.com --yes     # scripted (snapshot+audit still apply)
```

After a panic run, `flareout undo` reverses one record at a time.

### `flareout version`

Prints the binary version. Does NOT require a token (no network call).

## Where things go on disk

FlareOut writes operational state under `$XDG_STATE_HOME/flareout/`, falling back to `$HOME/.local/state/flareout/` when `$XDG_STATE_HOME` is unset.

| Path                                  | Purpose                                                 |
|---------------------------------------|---------------------------------------------------------|
| `flareout/debug.log`                  | Structured slog output (JSON) while the TUI is active   |
| `flareout/snapshots/<ts>-<zone>-<name>.json` | Pre-mutation record copy, written before any API write |
| `flareout/audit.jsonl`                | One JSON line per toggle attempt (success or failure)    |

Snapshots are mode `0600`, the directory `0700`. The token is **never** persisted to disk and never appears in any of these files (it is masked to `cfx...XXXX` in logs and stripped from error messages).

## Token Scope

FlareOut reads your Cloudflare API token from the `CLOUDFLARE_API_TOKEN` environment variable. There is no `--token` flag (intentional — flag values land in shell history).

| Operation                                  | Required scope                       |
|--------------------------------------------|--------------------------------------|
| `flareout list`, `flareout version`        | Zone:Read + DNS:Read                 |
| `flareout toggle --apply`, `undo`, `panic` | Zone:Edit + DNS:Edit                 |

**Cloudflare does not expose granted scopes in the token-verification API response.** FlareOut prints a non-suppressible warning at startup reminding you of this. If your token is missing a required scope, the first failing API call will surface a permission error from Cloudflare directly.

## Development

```sh
go test ./...                  # all tests, with the race detector in CI
go vet ./...
golangci-lint run              # v2 config; install via brew install golangci-lint
go build -o flareout ./cmd/flareout
```

The codebase follows clean / hexagonal architecture: `internal/domain` is stdlib-only and protected by an import-graph test, `internal/app` orchestrates use cases, `internal/infra` adapts to Cloudflare and the local filesystem, `internal/ui` covers the Cobra CLI and the Bubbletea TUI. The full project specs live in [`openspec/specs/`](openspec/specs/).

Releases are produced by [GoReleaser](https://goreleaser.com/) on every `v*` tag. Pre-release CI runs on every push and pull request to `main`.

## License

Apache 2.0 — see the [LICENSE](LICENSE) file for the full text.
