# Spec: Module Bootstrap

**Capability**: Module Bootstrap
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs the Go module initialization, Go version pinning, and project directory layout
invariants that every subsequent change inherits. These properties are verified statically
(file existence, `go.mod` content inspection) and via CI toolchain execution.

---

## Scenarios

### MB-01: go.mod declares Go 1.22 or higher

**Given** a clean checkout of the repository after the foundation change is applied

**When** `go.mod` is inspected for its `go` directive

**Then** the declared version MUST be `1.22` or higher (e.g. `go 1.22.0` or `go 1.23`)

**And** the module path MUST be `github.com/<owner>/flareout` (a non-empty, valid module path)

**Notes**:
- `log/slog` reached maturity in Go 1.21. Pinning `1.22+` ensures slog is available with no third-party substitutes.
- Declaring a lower version MUST be treated as a spec violation.

---

### MB-02: go.sum is present after module initialization

**Given** `go.mod` exists and all declared dependencies are fetched

**When** `go mod tidy` completes successfully

**Then** `go.sum` MUST exist at the repository root

**And** `go.sum` MUST be committed to the repository (not listed in `.gitignore`)

---

### MB-03: Project directory structure matches the layout contract

**Given** the foundation change is applied

**When** the repository directory tree is inspected

**Then** ALL of the following paths MUST exist:

| Path | Kind |
|------|------|
| `cmd/flareout/main.go` | file |
| `internal/domain/` | directory |
| `internal/app/` | directory |
| `internal/infra/cloudflare/` | directory |
| `internal/infra/config/` | directory |
| `internal/infra/logging/` | directory |
| `internal/ui/cli/` | directory |
| `internal/ui/tui/` | directory |

**And** `internal/ui/tui/` MUST contain at least a `doc.go` file declaring `package tui`
so `go vet ./...` does not flag an empty directory

---

### MB-04: main.go is a thin entrypoint (10–15 lines)

**Given** `cmd/flareout/main.go` exists

**When** its content is inspected

**Then** it MUST contain a `main()` function that delegates immediately to `cli.Execute()` or equivalent

**And** it MUST NOT contain business logic, flag parsing beyond the Cobra root, or direct API calls

**And** its total line count (including blank lines and package declaration) SHOULD NOT exceed 20 lines

---

### MB-05: go test ./... exits 0 from a clean checkout

**Given** a clean checkout of the repository with all dependencies available (`go mod download` has run)

**When** `go test ./...` is executed

**Then** the exit code MUST be 0

**And** at least one test MUST be reported as `PASS` (the `MaskToken` gate test in `internal/domain`)

---

### MB-06: go vet ./... exits 0

**Given** a clean checkout with all dependencies available

**When** `go vet ./...` is executed

**Then** the exit code MUST be 0

**And** no diagnostic messages MUST be emitted to stderr

---

### MB-07: go.mod Go version is not manually overridden by individual packages

**Given** the `go.mod` root declaration sets the minimum Go version

**When** any `//go:build` constraint or individual file is inspected

**Then** no file MUST declare a minimum Go version lower than the value in `go.mod`

**Notes**: This prevents silent local-toolchain downgrades from hiding incompatibilities.
