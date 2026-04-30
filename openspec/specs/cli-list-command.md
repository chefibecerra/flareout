# Spec: CLI List Command

**Capability**: CLI List Command (`flareout list`)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the `flareout list` Cobra subcommand, its `--json` flag, TTY auto-detection logic,
output routing, exit codes, and token-requirement annotation. This is the first user-visible
subcommand after `flareout version`. It exercises both the JSON writer path and the TUI launcher
path depending on the output environment.

---

## Scenarios

### CL-01: `flareout list` is registered on the root command

**Given** `internal/ui/cli/root.go` after the change is applied

**When** the root `*cobra.Command` is constructed via `NewRootCmd(deps)`

**Then** `list` MUST appear as a registered subcommand

**And** `flareout list --help` MUST succeed (exit 0) and display a usage line referencing
`flareout list [flags]`

**And** the help text MUST include a sentence describing the minimum required token scope
(DNS:Read)

---

### CL-02: `flareout list` has a `--json` boolean flag defaulting to false

**Given** the `list` subcommand is constructed

**When** `--json` flag is inspected

**Then** a boolean flag named `json` MUST be registered on the `list` command

**And** its default value MUST be `false`

**And** `flareout list --json` MUST set the flag to `true` for that invocation

---

### CL-03: `--json` flag forces JSON output even when stdout is a TTY

**Given** the test harness sets stdout to a PTY (simulated TTY, `isatty` returns `true`)
**And** the `--json` flag is set

**When** `flareout list --json` is executed

**Then** the output MUST be a valid JSON array on stdout

**And** the TUI MUST NOT be launched

**And** the process MUST exit with code 0

---

### CL-04: Non-TTY stdout without `--json` auto-selects JSON mode

**Given** stdout is a pipe (non-TTY, `isatty.IsTerminal(os.Stdout.Fd())` returns `false`)
**And** the `--json` flag is NOT set

**When** `flareout list` is executed

**Then** the output MUST be a valid JSON array on stdout

**And** the TUI MUST NOT be launched

**And** the process MUST exit with code 0

---

### CL-05: TTY stdout without `--json` launches TUI

**Given** stdout is a TTY (`isatty.IsTerminal(os.Stdout.Fd())` returns `true`)
**And** the `--json` flag is NOT set

**When** `flareout list` is executed

**Then** the TUI MUST be launched via `tea.NewProgram`

**And** JSON MUST NOT be written to stdout before the TUI starts

---

### CL-06: JSON output is a flat array parseable by jq

**Given** `flareout list --json` is executed against a fake `RecordLister` returning 2 records

**When** stdout is captured and parsed as JSON

**Then** the parsed value MUST be a JSON array with exactly 2 elements

**And** each element MUST be a JSON object containing the keys defined in RL-10
(`zone_id`, `zone_name`, `id`, `type`, `name`, `content`, `proxied`, `ttl`)

**And** `proxied` MUST be a boolean (never `null`)

---

### CL-07: `flareout list` does NOT carry `skip_verify` annotation

**Given** the `list` subcommand

**When** its `Annotations` map is inspected

**Then** the key `"skip_verify"` MUST NOT be present (or MUST NOT be set to `"true"`)

**And** the token verification middleware MUST execute before the list handler runs

**Notes**: A valid `CLOUDFLARE_API_TOKEN` is required to call the listing API. There is no
guest or anonymous mode.

---

### CL-08: API error in JSON mode prints error to stderr and exits 1

**Given** `flareout list --json` is executed
**And** the `RecordLister` returns an error from `ListZones`

**When** the command handler processes the error

**Then** a human-readable error message MUST be printed to stderr

**And** stdout MUST NOT contain a partial JSON array

**And** the process MUST exit with code 1

---

### CL-09: API error in JSON mode does not write partial JSON to stdout

**Given** `flareout list --json` is executed
**And** `ListZones` returns a non-nil error

**When** the command exits

**Then** stdout MUST be empty (no partial JSON written before the error was detected)

---

### CL-10: `flareout list --json` exits 0 on success

**Given** `flareout list --json` is executed
**And** the fake `RecordLister` returns records without error

**When** the command completes

**Then** the process MUST exit with code 0

---

### CL-11: JSON output is produced by json.Encoder on cmd.OutOrStdout()

**Given** the CLI list command implementation

**When** the JSON writer path is executed

**Then** output MUST be written via `json.NewEncoder(cmd.OutOrStdout()).Encode(records)`
(not `fmt.Println`, not direct `os.Stdout.Write`)

**And** tests MAY redirect `cmd.OutOrStdout()` to a `bytes.Buffer` to capture output without
spawning a subprocess

---

### CL-12: Context cancellation during JSON mode exits 1 with stderr message

**Given** `flareout list --json` is executing
**And** the context is cancelled while `app.ListAllRecords` is in-flight

**When** the cancellation is received

**Then** the command MUST print `"flareout: aborted"` (or equivalent) to stderr

**And** stdout MUST NOT contain partial JSON

**And** the process MUST exit with code 1, not 0

---

### CL-13: TUI log handler swap occurs before tea.NewProgram in the TUI path

**Given** the TUI path is selected (TTY, no `--json`)

**When** the list command handler executes

**Then** `logging.SwapToFileJSON(path)` MUST be called before `tea.NewProgram(...).Run()`

**And** `defer restore()` MUST be set immediately after the swap

**And** any `slog` writes from `app.ListAllRecords` during TUI operation MUST go to the log file,
not to stderr

**Notes**: This scenario extends LF-03 from the logging-foundation canonical spec. The
`flareout list` command is the first exerciser of that contract.
