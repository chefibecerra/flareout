# Spec: TUI List Screen

**Capability**: TUI List Screen (Bubbletea model)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the Bubbletea model in `internal/ui/tui/list_model.go`. The model is a finite state
machine with three states: `loading`, `loaded`, and `error`. It uses `bubbles/spinner` during
data fetch and `bubbles/table` once data is available.

Testing uses `teatest` with `WaitFor` + `bytes.Contains`. Golden-file comparison (byte-for-byte
ANSI output matching via `RequireEqualOutput`) is PROHIBITED for this model — ANSI escape codes
differ across terminal emulators and Bubbletea versions and produce false positives.

---

## Scenarios

### TS-01: Initial state is loading with a spinner

**Given** a new `list_model` is constructed via `list_model.New(appCtx)`

**When** the model's `Init()` method is called and the model renders its initial view

**Then** the model's internal state MUST be `loading`

**And** the rendered output MUST contain a non-empty spinner indicator (some rotating character
or progress text)

**And** a `tea.Cmd` MUST be returned from `Init()` that will trigger the data fetch

---

### TS-02: Model transitions from loading to loaded when data arrives

**Given** a `list_model` in `loading` state
**And** a `tea.Msg` carrying a successful `[]domain.Record` result is sent

**When** the model's `Update` method processes the message

**Then** the model's internal state MUST become `loaded`

**And** the `bubbles/table` component MUST be initialized with the record data

**And** `View()` MUST no longer render the spinner

---

### TS-03: Model transitions from loading to error when fetch fails

**Given** a `list_model` in `loading` state
**And** a `tea.Msg` carrying a non-nil error is sent to the model

**When** `Update` processes the error message

**Then** the model's internal state MUST become `error`

**And** `View()` MUST render an error banner containing the error message text

**And** `View()` MUST NOT render the table or the spinner

---

### TS-04: Error state displays a message that does not panic on quit

**Given** a `list_model` in `error` state rendering an error banner

**When** the user presses `q`

**Then** the model MUST return `tea.Quit` command

**And** no panic MUST occur

**And** the TUI program MUST exit cleanly

---

### TS-05: Table columns are Zone, Name, Type, Content, Proxied, TTL

**Given** a `list_model` in `loaded` state with at least one record

**When** the table header row is rendered

**Then** the rendered output MUST contain the column headers:
`Zone`, `Name`, `Type`, `Content`, `Proxied`, `TTL`

**And** the column headers MUST appear in that order (left to right)

---

### TS-06: Content column is truncated to approximately 30 characters

**Given** a record whose `Content` field is longer than 30 characters (e.g., `"a.very.long.dns.content.value.that.exceeds.thirty.characters"`)

**When** the TUI renders the table in `loaded` state

**Then** the `Content` cell for that record MUST display at most 30 characters of the content

**And** truncation MUST NOT cause a panic

**And** truncation MUST NOT corrupt adjacent columns

---

### TS-07: Proxied column displays [P] for true and [-] for false

**Given** a `list_model` in `loaded` state with two records:
- Record A: `Proxied = true`
- Record B: `Proxied = false`

**When** the table is rendered

**Then** the row for Record A MUST display `[P]` in the Proxied column

**And** the row for Record B MUST display `[-]` in the Proxied column

---

### TS-08: Arrow keys and j/k navigate the table

**Given** a `list_model` in `loaded` state with at least 3 records

**When** a `tea.KeyMsg` for the down arrow or `j` key is sent

**Then** the selected row MUST advance by one

**When** a `tea.KeyMsg` for the up arrow or `k` key is sent

**Then** the selected row MUST move back by one

**And** navigation MUST not wrap past the first or last row

---

### TS-09: q key quits the TUI from loaded state

**Given** a `list_model` in `loaded` state

**When** a `tea.KeyMsg` for `q` is sent

**Then** `Update` MUST return a `tea.Quit` command

**And** no panic MUST occur

---

### TS-10: Ctrl+C quits the TUI from any state

**Given** a `list_model` in any state (loading, loaded, or error)

**When** a `tea.KeyMsg` matching `ctrl+c` is sent

**Then** `Update` MUST return a `tea.Quit` command

**And** no panic MUST occur in any state

---

### TS-11: Esc key quits the TUI from any state

**Given** a `list_model` in any state

**When** a `tea.KeyMsg` for `esc` is sent

**Then** `Update` MUST return a `tea.Quit` command

---

### TS-12: tea.WindowSizeMsg adjusts table height

**Given** a `list_model` in `loaded` state

**When** a `tea.WindowSizeMsg{Width: 120, Height: 40}` is sent

**Then** the table height MUST be updated to fit within the new terminal height

**And** no panic MUST occur

**And** `View()` MUST return a non-empty string after the resize

---

### TS-13: tea.WithAltScreen() is required

**Given** the list command handler launches the TUI

**When** `tea.NewProgram(model, ...)` is called

**Then** `tea.WithAltScreen()` MUST be passed as a program option

**Notes**: This ensures the TUI renders in the alternate screen buffer and does not corrupt
the user's terminal scroll history on exit.

---

### TS-14: Slog handler MUST be swapped to file BEFORE tea.NewProgram().Run()

**Given** the list command is in the TUI path
**And** `logging.StateLogPath()` has been called to resolve the log file path

**When** the execution sequence of the list command handler is analyzed

**Then** `logging.SwapToFileJSON(path)` MUST be the LAST call before `tea.NewProgram(...).Run()`
among any setup operations

**And** `defer restore()` MUST be registered immediately after the swap

**And** no `slog` write that would go to stderr MUST occur between the swap and the TUI exit

**Notes**: This constraint is inherited from canonical spec LF-03 (logging-foundation).
`flareout list` is the first exerciser.

---

### TS-15: TUI test MUST NOT use RequireEqualOutput or golden file comparison

**Given** `internal/ui/tui/list_model_test.go` after the change is applied

**When** the test file is inspected

**Then** no call to `tm.RequireEqualOutput(t, ...)` MUST appear

**And** no golden file (fixture file with `.golden` or `.txt` extension used for byte comparison)
MUST be present in the `tui` package or its test data directory

**And** all TUI assertions MUST use `tm.WaitFor(...)` with `bytes.Contains` targeting
plain-text substrings

---

### TS-16: TUI loading state test detects spinner text

**Given** a `teatest.TestModel` wrapping a fresh `list_model.New(fakeAppCtx)`

**When** `tm.WaitFor(func(b []byte) bool { ... })` is called immediately after construction

**Then** the waiting function MUST be able to detect a spinner indicator character or the
loading status text within the initial frames

**And** the `WaitFor` MUST complete without timeout on a correctly implemented model

---

### TS-17: TUI loaded state test detects table header without golden comparison

**Given** a `teatest.TestModel` wrapping a `list_model`
**And** a `tea.Msg` carrying a successful record list is sent via `tm.Send(...)`

**When** `tm.WaitFor(func(b []byte) bool { return bytes.Contains(b, []byte("Zone")) })` is called

**Then** the `WaitFor` MUST complete (model transitioned to loaded and rendered the table header)

**And** no golden file comparison MUST be used

---

### TS-18: TUI error state test detects error text without golden comparison

**Given** a `teatest.TestModel` wrapping a `list_model`
**And** a `tea.Msg` carrying a non-nil error with message `"connection refused"` is sent

**When** `tm.WaitFor(func(b []byte) bool { return bytes.Contains(b, []byte("connection refused")) })`
is called

**Then** the `WaitFor` MUST complete (error banner is rendered with the error text)

**And** no golden file comparison MUST be used
