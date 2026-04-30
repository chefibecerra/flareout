# Spec: Signal Handling

**Capability**: Signal Handling (SIGINT / SIGTERM propagation)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the `signal.NotifyContext` wiring in `cmd/flareout/main.go` and the observable
behavior when the user presses Ctrl-C or the process receives SIGTERM during `flareout list`.
This change introduces the first long-running network operation in FlareOut; clean
cancellation is a safety property, not a nice-to-have.

The context created by `signal.NotifyContext` is passed to `root.ExecuteContext(ctx)`, which
Cobra propagates to all subcommand handlers. Subcommands do not need to know about signal
handling — they only work with `context.Context`.

---

## Scenarios

### SH-01: main.go uses signal.NotifyContext instead of context.Background()

**Given** `cmd/flareout/main.go` after the change is applied

**When** the source of the `main` function is inspected

**Then** `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` MUST appear

**And** the returned context MUST be passed to `root.ExecuteContext(ctx)` (not a different context)

**And** `context.Background()` MUST NOT be passed directly to `root.ExecuteContext`

---

### SH-02: stop function from signal.NotifyContext is deferred

**Given** `cmd/flareout/main.go` after the change is applied

**When** the source of the `main` function is inspected

**Then** `defer stop()` MUST appear immediately after `signal.NotifyContext(...)` is called

**Notes**: The deferred `stop()` releases the signal handler on normal (non-signal) program exit,
preventing goroutine leaks in the OS signal notification infrastructure.

---

### SH-03: Ctrl-C during JSON mode loading exits 1 — not 0

**Given** `flareout list --json` is executing
**And** `app.ListAllRecords` is in-flight (network request pending)
**And** the user sends SIGINT (Ctrl-C)

**When** the signal propagates through the context

**Then** the process MUST exit with code 1

**And** the process MUST NOT exit with code 0

---

### SH-04: Ctrl-C during JSON mode does not produce a panic

**Given** `flareout list --json` is executing
**And** the context is cancelled via SIGINT

**When** the cancellation reaches `app.ListAllRecords` and propagates through `errgroup`

**Then** no panic MUST occur in any goroutine

**And** the program MUST terminate cleanly via `os.Exit(1)` or equivalent Cobra exit code

---

### SH-05: Ctrl-C during JSON mode prints an aborted message to stderr

**Given** `flareout list --json` is executing
**And** SIGINT is received during data fetch

**When** the command handler detects the context error

**Then** a non-empty message MUST be printed to stderr (e.g., `"flareout: aborted"` or
the error string from the context)

**And** stdout MUST NOT contain any partial JSON output

---

### SH-06: Ctrl-C during TUI loading quits the TUI cleanly

**Given** `flareout list` is executing in TUI mode (TTY, no `--json`)
**And** the model is in `loading` state
**And** the user presses Ctrl-C

**When** the TUI program receives the quit signal

**Then** the TUI MUST exit cleanly (no raw terminal state left behind)

**And** the alt-screen MUST be restored

**And** no panic MUST occur

---

### SH-07: Ctrl-C during TUI loading results in exit code 1

**Given** `flareout list` is executing in TUI mode
**And** Ctrl-C is pressed while still loading

**When** the program exits

**Then** the exit code MUST be 1, not 0

**Notes**: An aborted operation is not a success. Exit code 1 allows shell scripts to detect
the abort condition.

---

### SH-08: SIGTERM is treated identically to SIGINT

**Given** `flareout list --json` is executing

**When** the process receives SIGTERM (e.g., from `kill <pid>`)

**Then** the behavior MUST be identical to SIGINT: context cancelled, exit code 1, no panic,
error message on stderr

---

### SH-09: Existing Cobra tests are not broken by signal.NotifyContext in main.go

**Given** existing tests in `internal/ui/cli/` that call `root.ExecuteContext(context.Background())`

**When** those tests run after the `main.go` change is applied

**Then** all existing tests MUST continue to pass

**And** no test MUST need to be updated to supply a signal-aware context

**Notes**: `signal.NotifyContext` is wired in `main.go` only. `NewRootCmd` and all command
handlers accept a generic `context.Context`. Tests continue to pass any context they want.

---

### SH-10: stop() is called before process exits on normal completion

**Given** `flareout list --json` completes successfully (no signal)

**When** the program exits

**Then** the deferred `stop()` MUST be called before `os.Exit` is reached

**Notes**: This is verified structurally by the presence of `defer stop()` in main.go. A unit
test MAY verify the wiring by inspecting the source; a full signal test is not feasible in
`go test` without subprocess execution.
