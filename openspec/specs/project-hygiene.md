# Spec: Project Hygiene

**Capability**: Project Hygiene
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs the repository-level files that establish legal standing (LICENSE), project
identity (README), repository cleanliness (.gitignore), code quality gate (.golangci.yml),
and automated CI (.github/workflows/ci.yml). These are binary-verifiable: each file
either exists with the required content or it does not.

---

## Scenarios

### PH-01: LICENSE contains the full Apache 2.0 text

**Given** the foundation change is applied

**When** the file `LICENSE` at the repository root is inspected

**Then** `LICENSE` MUST exist

**And** it MUST contain the full Apache 2.0 license text, including the standard header:
`Apache License, Version 2.0`

**And** it MUST NOT be a stub, a link, or a placeholder — the full canonical text is required

**And** the copyright year and holder MUST be filled in (not left as `[year]` or `[owner]`
template placeholders)

---

### PH-02: README.md exists with required content

**Given** the foundation change is applied

**When** `README.md` at the repository root is inspected

**Then** `README.md` MUST exist

**And** it MUST contain the project name `FlareOut` (or equivalent heading)

**And** it MUST contain a one-paragraph description of what FlareOut does

**And** it MUST contain an install placeholder section (even if the install method is TBD)

**And** it MUST contain a license badge or license section referencing Apache 2.0

**And** it MUST contain a section on token scope (explaining the scope-validation gap
and the documentation-only warning) — this directly supports the non-suppressible
warning's UX story

---

### PH-03: .gitignore covers Go binaries, vendor, IDE noise, and .flareout/

**Given** the foundation change is applied

**When** `.gitignore` at the repository root is inspected

**Then** `.gitignore` MUST exist

**And** it MUST ignore compiled Go binaries (e.g. the `flareout` binary itself)

**And** it MUST ignore the `vendor/` directory

**And** it MUST ignore common IDE/editor directories and files (e.g. `.idea/`, `.vscode/`,
`*.swp`)

**And** it MUST ignore `.flareout/` (the future log and config directory for the tool itself)

**And** `go.sum` MUST NOT be listed in `.gitignore` (it MUST be committed)

---

### PH-04: .golangci.yml enables the curated linter set

**Given** `.golangci.yml` at the repository root

**When** its content is inspected

**Then** `.golangci.yml` MUST exist

**And** it MUST enable at minimum these linters:
- `errcheck`
- `govet`
- `staticcheck`
- `unused`
- `gofmt`
- `revive`

**And** it MUST NOT enable style-only linters that produce excessive noise during early
development (e.g. `wsl`, `nlreturn`, `godot`)

**And** `golangci-lint run` executed against the codebase MUST exit 0 with this configuration

---

### PH-05: CI workflow file exists with test and lint jobs

**Given** `.github/workflows/ci.yml`

**When** its content is inspected

**Then** `.github/workflows/ci.yml` MUST exist

**And** it MUST define a `test` job that runs BOTH:
- `go vet ./...`
- `go test ./...`

**And** it MUST define a `lint` job that runs `golangci-lint` via `golangci-lint-action@v6`
or a version-pinned equivalent

**And** both jobs MUST be triggered on `push` and `pull_request` events

**And** the workflow MUST use `go-version: stable` (or a pinned Go version >= 1.22) via
`actions/setup-go`

---

### PH-06: CI lint job uses the curated golangci-lint configuration

**Given** `.github/workflows/ci.yml` and `.golangci.yml`

**When** the lint job configuration is inspected

**Then** the lint job MUST reference `.golangci.yml` for linter selection
(either by default discovery or via explicit `--config` flag)

**And** the golangci-lint version MUST be declared (MAY be `latest` in foundation;
SHOULD be pinned to a specific version before v1.0.0 tagging)

---

### PH-07: CI passes on a clean checkout

**Given** a clean checkout of the repository after the foundation change is applied

**When** both CI jobs (`test` and `lint`) execute

**Then** both MUST exit with code `0`

**And** no test failures, vet errors, or lint violations MUST be reported
