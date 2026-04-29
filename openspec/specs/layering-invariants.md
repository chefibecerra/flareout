# Spec: Layering Invariants

**Capability**: Layering Invariants
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs the import graph rules that enforce clean/hexagonal architecture. These rules
are not stylistic — they are structural correctness properties. A violation means the
architecture boundary has leaked, which degrades testability and makes future changes
harder to isolate.

The key invariant: `internal/domain` is the innermost layer and MUST have no dependencies
on any other internal layer. All other layers depend inward.

The method of enforcement (static import-graph analysis, `go list -deps`, a dedicated
layering test, or lint rule) is a design-phase decision. This spec only requires the
property to hold; it does not prescribe the implementation of the check.

---

## Scenarios

### LI-01: internal/domain imports nothing from app, infra, or ui

**Given** the codebase compiles after the foundation change is applied

**When** the import graph of every Go package under `internal/domain/` is analyzed

**Then** no package in `internal/domain/` MUST import any package from:
- `internal/app/`
- `internal/infra/`
- `internal/ui/`

**And** packages in `internal/domain/` MAY only import from:
- Go standard library packages
- Third-party packages that are pure value types or well-known interfaces (e.g. `context`, `time`)
- Other packages within `internal/domain/` itself

**Notes**: `internal/domain` is the contract. It is allowed to use stdlib and context — it
MUST NOT know that Cloudflare, Cobra, or Bubbletea exist.

---

### LI-02: internal/app imports only domain (not infra or ui)

**Given** the codebase compiles

**When** the import graph of every package under `internal/app/` is analyzed

**Then** no package in `internal/app/` MUST import any package from:
- `internal/infra/`
- `internal/ui/`

**And** `internal/app/` MAY import `internal/domain/`

**Notes**: Use cases (app layer) depend on port interfaces, not on adapters. This keeps
use cases testable with fakes without needing a real Cloudflare client.

---

### LI-03: internal/infra imports domain (to satisfy ports) but not ui

**Given** the codebase compiles

**When** the import graph of every package under `internal/infra/` is analyzed

**Then** no package in `internal/infra/` MUST import any package from `internal/ui/`

**And** `internal/infra/` MAY import `internal/domain/` (required to implement port interfaces)

**And** `internal/infra/` MUST NOT import `internal/app/` (adapters are constructed at the
composition root, not from within use cases)

---

### LI-04: internal/ui imports app and domain but not infra directly

**Given** the codebase compiles

**When** the import graph of every package under `internal/ui/` is analyzed

**Then** packages in `internal/ui/` SHOULD NOT import `internal/infra/` directly

**And** adapter construction MUST occur at the composition root (`cmd/flareout/main.go`
or a `wiring.go` helper), not inside UI packages

**Notes**: This is SHOULD rather than MUST because the composition root may be realized
as a helper inside `internal/app/wiring.go`. The intent is that UI packages receive
already-constructed use case instances; they do not know which infra adapter backs them.

---

### LI-05: Composition root is the only place that knows all layers

**Given** the source code of `cmd/flareout/main.go` and any wiring helper it delegates to

**When** the imports of the composition root are inspected

**Then** the composition root MAY import from ALL layers (`domain`, `app`, `infra`, `ui`)
because it is the wiring point

**And** no other file outside the composition root MAY import from more than two adjacent
layers simultaneously

---

### LI-06: Layering property is verifiable (static or test-based)

**Given** the foundation change is applied

**When** a reviewer or CI system validates the layering invariants

**Then** at minimum ONE of the following MUST be true:
1. A Go test exists (e.g. `internal/domain/layering_test.go`) that uses `go list -deps`
   or `golang.org/x/tools/go/packages` to assert the import graph constraints
2. A `revive` or custom lint rule is configured in `.golangci.yml` that enforces the
   layer boundaries
3. The CI `lint` job catches violations without manual inspection

**Notes**: Design phase SHOULD choose which mechanism to use. Foundation MUST at minimum
document this as a requirement. If neither a lint rule nor a test is feasible in foundation,
the layering MUST still hold by construction and be verifiable by manual import inspection.
The spec does not block on a specific verification mechanism, but it MUST NOT be left
entirely implicit.

---

### LI-07: cloudflare-go is imported only within internal/infra/cloudflare

**Given** the full Go source of the repository

**When** all import statements are inspected for `github.com/cloudflare/cloudflare-go`

**Then** that dependency MUST appear ONLY within `internal/infra/cloudflare/`

**And** it MUST NOT be imported in `internal/domain/`, `internal/app/`, `internal/ui/`,
or `cmd/flareout/`

**Notes**: Scattering the Cloudflare SDK across layers creates adapter lock-in that makes
future mock/fake strategies painful. A single adapter package is the hard boundary.
