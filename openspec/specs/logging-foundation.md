# Spec: Logging Foundation

**Capability**: Logging Foundation
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs the `log/slog` setup, handler configuration for CLI vs TUI modes, and the
`SwapToFileJSON` helper contract. The critical constraint driving this capability is
that any `slog` write to stderr while Bubbletea owns the terminal corrupts TUI rendering.
The handler-swap helper is the single mechanism that prevents this.

Foundation does not launch any TUI screen; the helper is implemented and its contract
is formally specified here so downstream changes (`flareout-list` onward) inherit a
stable, documented calling convention.

---

## Scenarios

### LF-01: Default CLI handler writes text to stderr

**Given** the application starts in CLI mode (any non-TUI command, including `flareout version`)

**When** `slog.Default()` is inspected after initialization

**Then** the default logger MUST use `slog.NewTextHandler` writing to `os.Stderr`

**And** the minimum log level MUST be `slog.LevelInfo` (debug messages MUST NOT appear
in normal CLI output)

**And** no JSON output MUST appear on stderr during CLI mode operation

---

### LF-02: SwapToFileJSON exists and is exported from internal/infra/logging

**Given** the foundation change is applied

**When** `internal/infra/logging/logging.go` is inspected

**Then** a function named `SwapToFileJSON` MUST be exported from the `logging` package

**And** its signature MUST accept an `io.Writer` parameter

**And** it MUST call `slog.SetDefault` with a new `slog.Logger` backed by
`slog.NewJSONHandler(w, ...)` where `w` is the provided writer

**And** the function MUST be callable from outside the package (exported, not `swapToFileJSON`)

---

### LF-03: SwapToFileJSON MUST be called before tea.NewProgram().Run()

**Given** a command that launches a Bubbletea TUI screen (note: foundation has no TUI command;
this scenario specifies the contract for downstream changes)

**When** the execution flow for that command is analyzed

**Then** `logging.SwapToFileJSON(file)` MUST be called BEFORE `tea.NewProgram(model).Run()`

**And** no `slog` write to `os.Stderr` MUST occur between the call to `SwapToFileJSON`
and the exit of `tea.NewProgram(model).Run()`

**Notes**: Violation of this ordering guarantee corrupts TUI rendering. This is a MUST-level
correctness requirement for every TUI command in every subsequent change. Foundation
documents the contract; `flareout-list` is the first exerciser.

---

### LF-04: CLI commands do not call SwapToFileJSON

**Given** any CLI command (non-TUI) in the foundation change (e.g., `flareout version`)

**When** its execution flow is analyzed

**Then** `logging.SwapToFileJSON` MUST NOT be called

**And** the default `TextHandler` on stderr MUST remain active for the full lifecycle
of a CLI command

---

### LF-05: No slog handler is constructed outside internal/infra/logging

**Given** any Go source file in the repository (excluding `internal/infra/logging/`)

**When** the source is inspected for calls to `slog.NewTextHandler` or `slog.NewJSONHandler`

**Then** no such calls MUST appear outside the `internal/infra/logging` package

**Notes**: Centralizing handler construction prevents ad-hoc handler choices from
proliferating across the codebase. `revive` or a code-review checklist SHOULD enforce
this. Design phase SHOULD determine whether a lint rule can catch this automatically.

---

### LF-06: DefaultCLI() helper exists and is exported

**Given** `internal/infra/logging/logging.go`

**When** its source is inspected

**Then** a function named `DefaultCLI` (or equivalent) MUST be exported

**And** it MUST return a `*slog.Logger` configured with `TextHandler` on `os.Stderr`
at `LevelInfo`

**And** it MUST be called at application startup (in `cmd/flareout/main.go` or equivalent)
to establish the default logger before any other code runs

---

### LF-07: Token value does not appear in any log output

**Given** a valid token is loaded from `CLOUDFLARE_API_TOKEN`

**When** any `slog` log statement anywhere in the codebase executes during a command run

**Then** the raw token value MUST NOT appear in the output of that log statement

**And** any log statement that references the token MUST use `MaskToken(t)` to produce
the `cfx...last4` masked form

**Notes**: This scenario overlaps with TL-03 intentionally. Logging Foundation reaffirms
the no-token-leakage property from the logging layer's perspective. Both specs MUST agree.
