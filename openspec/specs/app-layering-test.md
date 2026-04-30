# Spec: App Layering Test

**Capability**: App Package Layering Invariant Test
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the new static import-graph analysis test in `internal/app/layering_test.go`.
This test extends the enforcement mechanism established by the domain layering test in
`internal/domain/` (canonical spec LI-02, LI-06) to cover the `internal/app` package.

The test MUST use the same `go/parser`-based mechanism as the existing domain layering test
and MUST read the module path from `go.mod` at runtime (not hardcoded) to remain portable.

---

## Scenarios

### AL-01: Layering test file exists at the correct path

**Given** the change is applied

**When** the filesystem is inspected

**Then** the file `internal/app/layering_test.go` MUST exist

**And** it MUST declare `package app` (same package, not `package app_test`)

---

### AL-02: Test rejects any file in internal/app that imports internal/infra

**Given** `internal/app/layering_test.go` is running
**And** a hypothetical source file `internal/app/example.go` contains an import of
`internal/infra/cloudflare` or any other `internal/infra/...` subpackage

**When** the layering test executes

**Then** the test MUST fail with a message identifying the offending file and the
prohibited import path

---

### AL-03: Test rejects any file in internal/app that imports internal/ui

**Given** a hypothetical source file `internal/app/example.go` contains an import of
`internal/ui/cli` or any other `internal/ui/...` subpackage

**When** the layering test executes

**Then** the test MUST fail with a message identifying the offending file and the
prohibited import path

---

### AL-04: Test exempts wiring.go — and ONLY wiring.go — from layering checks

**Given** `internal/app/wiring.go` imports `internal/infra/cloudflare` (which is required
for constructing the `cloudflare.RecordLister` and assigning it to `Context.Lister`)

**When** the layering test executes

**Then** the test MUST NOT fail for `wiring.go` despite its `internal/infra` import

**And** the exemption MUST apply to `wiring.go` by filename match ONLY

**And** no other filename MUST be exempted automatically (no wildcard or suffix match)

---

### AL-05: Test exempts ONLY wiring.go — other files named similarly are NOT exempt

**Given** a hypothetical file `internal/app/rewiring.go` imports `internal/infra/cloudflare`

**When** the layering test executes

**Then** the test MUST fail for `rewiring.go`

**Notes**: The exemption check MUST use exact filename comparison (`filepath.Base(path) == "wiring.go"`),
not prefix or suffix matching.

---

### AL-06: Module path is read from go.mod at runtime

**Given** the layering test is executing

**When** it constructs the prohibited import path prefixes to check against

**Then** the module path (e.g., `github.com/user/flareout`) MUST be read from `go.mod`
at test runtime (not hardcoded as a string literal in the test source)

**And** the mechanism MUST mirror the pattern used in the existing domain layering test

**Notes**: Hardcoding the module path makes the test fragile to renames. Reading from
`go.mod` keeps it portable.

---

### AL-07: Test passes cleanly when no prohibited imports exist

**Given** the current state of `internal/app/` after the change is applied
**And** only `wiring.go` imports `internal/infra`
**And** no file in `internal/app/` imports `internal/ui`

**When** `go test ./internal/app/...` is run

**Then** the layering test MUST pass (exit 0)

**And** no false positive MUST be reported

---

### AL-08: Test uses go/parser to walk source files

**Given** the layering test implementation

**When** the test source is inspected

**Then** it MUST use `go/parser.ParseFile` or equivalent `go/parser` or
`golang.org/x/tools/go/packages` API to enumerate import declarations

**And** it MUST walk all `.go` files in `internal/app/` (not `go list -deps` subprocess)

**Notes**: Consistent with the pattern used by the domain layering test. Using the same
mechanism ensures the two tests are comprehensible side-by-side and do not diverge in
approach.

---

### AL-09: Test is located in the same package (not _test package)

**Given** `internal/app/layering_test.go`

**When** the package declaration is inspected

**Then** it MUST be `package app` (not `package app_test`)

**Notes**: Being in the same package is consistent with the domain layering test convention
and allows direct access to unexported helpers if needed. The test itself does not call any
unexported app functions — it only analyzes source files.
