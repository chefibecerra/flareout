# Spec: XDG Log Path

**Capability**: XDG Log Path (`logging.StateLogPath`)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs `logging.StateLogPath() (string, error)` in
`internal/infra/logging/logpath.go`. This helper resolves the debug log file path
using the XDG Base Directory specification for state files. It MUST create the
directory before returning the path so callers do not need to handle a missing directory.

This helper is called by the `flareout list` TUI path before `logging.SwapToFileJSON`
is invoked.

---

## Scenarios

### XL-01: XDG_STATE_HOME env var takes precedence over HOME-based fallback

**Given** the environment variable `XDG_STATE_HOME` is set to `/custom/state`

**When** `logging.StateLogPath()` is called

**Then** the returned path MUST be `/custom/state/flareout/debug.log`

**And** the returned error MUST be `nil`

---

### XL-02: Fallback to HOME-based path when XDG_STATE_HOME is not set

**Given** `XDG_STATE_HOME` is unset (empty string or not present in the environment)
**And** `os.UserHomeDir()` returns `/home/testuser`

**When** `logging.StateLogPath()` is called

**Then** the returned path MUST be `/home/testuser/.local/state/flareout/debug.log`

**And** the returned error MUST be `nil`

---

### XL-03: Directory is created with mode 0700 before the path is returned

**Given** the resolved directory (`$XDG_STATE_HOME/flareout` or `~/.local/state/flareout`)
does not yet exist on disk

**When** `logging.StateLogPath()` is called

**Then** the directory MUST be created (including any missing parent directories) via
`os.MkdirAll(dir, 0700)`

**And** the created directory MUST have permission mode `0700` (owner read/write/execute only)

**And** the returned path string MUST point inside the created directory

---

### XL-04: MkdirAll is idempotent — existing directory does not cause an error

**Given** the resolved directory already exists on disk

**When** `logging.StateLogPath()` is called

**Then** the returned error MUST be `nil`

**And** the existing directory MUST NOT be modified (permissions or contents unchanged)

---

### XL-05: Log file opened by SwapToFileJSON uses mode 0600

**Given** `logging.StateLogPath()` returns a valid path

**When** the caller opens or creates the log file at that path (via `logging.SwapToFileJSON`
or equivalent)

**Then** the file MUST be created with permissions `0600` (owner read/write only)

**Notes**: File mode enforcement is in the responsibility of the function that opens the file
(i.e., `SwapToFileJSON` or its successor). `StateLogPath` is responsible only for directory
creation. This scenario exists to document the full permission story in one place.

---

### XL-06: Error returned when HOME cannot be resolved and XDG_STATE_HOME is unset

**Given** `XDG_STATE_HOME` is unset
**And** `os.UserHomeDir()` returns an error (e.g., no home directory configured)

**When** `logging.StateLogPath()` is called

**Then** the returned path MUST be an empty string

**And** the returned error MUST be non-nil

**And** the error MUST include context indicating home directory resolution failed

---

### XL-07: XDG_STATE_HOME set to empty string is treated as unset

**Given** `XDG_STATE_HOME` is set to an empty string `""`

**When** `logging.StateLogPath()` is called

**Then** the function MUST treat the empty string the same as unset and fall back to the
`$HOME/.local/state/flareout/debug.log` path

**Notes**: `os.Getenv("XDG_STATE_HOME")` returns `""` for both unset and set-to-empty.
The implementation MUST check `!= ""` before using the env var value.

---

### XL-08: StateLogPath is exported from the logging package

**Given** `internal/infra/logging/logpath.go` after the change is applied

**When** the package's exported symbols are inspected

**Then** `StateLogPath` MUST be exported (capitalized) and callable from outside the package

**And** its signature MUST be `func StateLogPath() (string, error)` with no parameters

---

### XL-09: StateLogPath does not write to slog during normal operation

**Given** a test that intercepts `slog.Default()` output

**When** `logging.StateLogPath()` is called successfully

**Then** no slog message MUST be emitted by `StateLogPath` itself

**Notes**: Log path resolution is infrastructure setup, not an application event. Emitting a
log entry before the log file is established creates a bootstrap-ordering hazard.
