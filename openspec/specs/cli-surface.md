# Spec: CLI Surface

**Capability**: CLI Surface
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs the Cobra root command wiring, the `flareout version` subcommand, exit codes,
and help output. The version command is the proof-of-wiring signal that confirms the
Cobra tree is assembled correctly end-to-end without requiring a live Cloudflare token.

---

## Scenarios

### CS-01: flareout version prints a non-empty version string and exits 0

**Given** the binary is built from this change

**When** `flareout version` is executed with `CLOUDFLARE_API_TOKEN` unset

**Then** a non-empty version string MUST be printed to stdout

**And** the process MUST exit with code `0`

**And** the output MUST NOT contain the word `undefined` or be a bare `0.0.0` without context

**Notes**: The exact version string format (semver, `dev`, git SHA) is a design-phase decision.
The spec only requires it to be non-empty and meaningful. `flareout version` MUST NOT trigger
the token liveness check — it is a metadata command.

---

### CS-02: flareout root command help is available

**Given** the binary is built

**When** `flareout --help` or `flareout -h` is executed

**Then** a help message MUST be printed to stdout

**And** the help message MUST list `version` as an available subcommand

**And** the process MUST exit with code `0`

---

### CS-03: Unknown subcommand exits non-zero with a useful message

**Given** the binary is built

**When** `flareout unknowncmd` is executed (a subcommand that does not exist)

**Then** the process MUST exit with a non-zero exit code

**And** an error message MUST be printed indicating the subcommand is unknown

**And** the error message SHOULD suggest running `flareout --help` for available commands

---

### CS-04: Cobra root has no business logic

**Given** the Cobra root command definition in `internal/ui/cli/root.go`

**When** its source is inspected

**Then** it MUST NOT contain direct calls to the Cloudflare API

**And** it MUST NOT contain token loading or token validation logic inline

**And** token loading and validation MUST be triggered through a `PersistentPreRunE` hook
or equivalent delegation to the composition root, NOT hardcoded in any subcommand's
`RunE` body (except `flareout version`, which MAY skip the token check)

---

### CS-05: flareout version does not require CLOUDFLARE_API_TOKEN

**Given** `CLOUDFLARE_API_TOKEN` is NOT set in the environment

**When** `flareout version` is executed

**Then** the process MUST exit with code `0`

**And** no error about a missing token MUST be printed

**And** no token liveness check MUST be attempted

**Notes**: This is a deliberate exception — version is an introspection command and MUST
remain runnable without credentials. Every other command that interacts with Cloudflare
MUST enforce the token requirement.

---

### CS-06: --token flag does not exist

**Given** the CLI surface of the `flareout` binary

**When** `flareout --help` output is inspected for a `--token` flag

**Then** no `--token` (or `-t` shorthand for a token) flag MUST be present at any level
of the command tree

**Notes**: A `--token` flag would land in shell history and process listings, leaking the
token value. This is a permanent design decision, not a v1 shortcut. The flag MUST NOT be
added in any future change without an explicit security review and spec amendment.
