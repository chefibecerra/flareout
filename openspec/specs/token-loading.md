# Spec: Token Loading & Verification

**Capability**: Token Loading & Verification
**Change**: flareout-foundation
**Phase**: sdd-spec
**Date**: 2026-04-28

---

## Overview

Governs how FlareOut reads the Cloudflare API token, validates it, masks it in all
observable outputs, and communicates failures. Two critical constraints drive this
capability: (1) the token MUST NEVER appear in logs, error messages, or any printed
output; (2) the scope-validation gap — `VerifyAPIToken` returns only liveness, not
permissions — means scope enforcement is documentation-only in v1.

---

## Scenarios

### TL-01: Token is loaded exclusively from CLOUDFLARE_API_TOKEN env var

**Given** the foundation change is applied and any command that requires a token is invoked

**When** the composition root initializes the token loading flow

**Then** the token MUST be read ONLY from `os.Getenv("CLOUDFLARE_API_TOKEN")`

**And** no config file path (`~/.flareout/config.yaml` or equivalent) MUST be consulted
in this change

**And** no `--token` CLI flag MUST be accepted as a token source

---

### TL-02: Missing token produces a clear error with remediation hint

**Given** `CLOUDFLARE_API_TOKEN` is NOT set in the environment (empty string or absent)

**When** any command that requires a token runs (any command except `flareout version`)

**Then** the process MUST exit with a non-zero exit code

**And** an error message MUST be emitted that:
- States `CLOUDFLARE_API_TOKEN` is not set (by name)
- Includes a URL pointing to `https://dash.cloudflare.com/profile/api-tokens`
- Mentions the required DNS:Read scope

**And** the error message MUST NOT contain the token value (it is empty, but a defensive
check MUST ensure no accidental value interpolation occurs)

**And** no token liveness check MUST be attempted when the token is missing

---

### TL-03: Token value NEVER appears in any log line

**Given** a valid token (e.g. `abc1234567890abcdef1234`) is set in `CLOUDFLARE_API_TOKEN`

**When** any operation during the startup flow (token load, liveness check, warning emission,
or any subsequent command execution) produces log output via `slog`

**Then** the raw token string MUST NOT appear in any log line written to stderr or to a log file

**And** the token MUST NOT appear in any `slog` field value, key, or message string

**Notes**: This is a MUST-level security requirement. Violations are not warnings — they are
blocker defects. Code review MUST treat any `slog` call with a `"token"` key that does not
wrap its value in `MaskToken` as a spec violation.

---

### TL-04: Token value NEVER appears in error messages or printed output

**Given** a valid or invalid token is set in `CLOUDFLARE_API_TOKEN`

**When** any error message, `fmt.Fprintf`, `fmt.Println`, or equivalent is produced

**Then** the raw token string MUST NOT appear in that output

**And** if the token must be referenced in an error message (e.g. to show partial identity),
ONLY the masked form `cfx...last4` MAY appear

---

### TL-05: MaskToken returns the cfx...last4 form for valid tokens

**Given** a token string `s` with length >= 4

**When** `MaskToken(s)` is called

**Then** the return value MUST be the string `"cfx..."` concatenated with the last 4 characters of `s`

**Example**: `MaskToken("abc1234567890abcdef1234")` MUST return `"cfx...1234"`

---

### TL-06: MaskToken handles edge cases safely

**Given** the following inputs to `MaskToken`:

| Input | Expected output |
|-------|----------------|
| `""` (empty string) | `"cfx...[empty]"` or `"cfx..."` — a non-empty, non-leaking placeholder |
| `"abc"` (length 3, less than 4 chars) | `"cfx...abc"` or a safe truncation — MUST NOT panic |
| `"abcd"` (exactly 4 chars) | `"cfx...abcd"` |
| `"abcde"` (5 chars) | `"cfx...bcde"` |

**Then** `MaskToken` MUST NOT panic for any string input

**And** `MaskToken` MUST NOT return the full token value for any input of length >= 4

**Notes**: These exact cases MUST be covered by `internal/domain/token_test.go`.
This is the gate test whose passage activates strict TDD mode for subsequent changes.

---

### TL-07: Liveness check succeeds — success path with warning emission

**Given** `CLOUDFLARE_API_TOKEN` is set to a token that `VerifyAPIToken` accepts as active

**When** the startup flow calls `TokenVerifier.Verify(ctx)`

**Then** the verification MUST succeed (no error returned)

**And** a success log line MUST be emitted via `slog.Info` containing:
- The key `"token"` with value `MaskToken(t)` (masked form only)
- The key `"expires_at"` with the expiry value from `TokenStatus`

**And** the documentation scope warning MUST be emitted to stderr (see TL-09)

**And** the process MUST continue to execute the requested command

---

### TL-08: Liveness check fails — token rejected by VerifyAPIToken

**Given** `CLOUDFLARE_API_TOKEN` is set to a token that `VerifyAPIToken` rejects
(expired, revoked, or invalid format)

**When** the startup flow calls `TokenVerifier.Verify(ctx)`

**Then** the verification MUST return a non-nil error

**And** an error log line MUST be emitted via `slog.Error` containing:
- The key `"token"` with value `MaskToken(t)` (masked form only, NEVER raw)
- The key `"err"` with the underlying error

**And** the process MUST exit with a non-zero exit code BEFORE any user-facing command body runs

**And** the documentation scope warning MUST NOT be emitted on the failure path (warning is
success-path-only per proposal section 4.4)

---

### TL-09: Documentation scope warning is emitted on every successful verification

**Given** `CLOUDFLARE_API_TOKEN` is set to a token that passes liveness check

**When** the startup flow completes token verification successfully

**Then** the following warning MUST be printed to stderr:

```
FlareOut requires only DNS:Read zone-level permissions. Cloudflare's API does not
expose token scope, so FlareOut cannot enforce least privilege automatically.
Consider using a dedicated read-only token: https://dash.cloudflare.com/profile/api-tokens
```

(Exact wording MAY vary in design/apply phases; the semantic content MUST match.)

**And** this warning MUST be emitted exactly ONCE per invocation

**And** this warning MUST appear BEFORE the requested command body executes

---

### TL-10: Documentation scope warning is non-suppressible

**Given** any version of the CLI surface in this change

**When** the complete flag, env var, and config surface is inspected

**Then** there MUST NOT exist any flag, environment variable, or configuration option
that suppresses or silences the scope warning

**Notes**: This is a permanent v1 constraint, not a shortcut. A future change that adds
a suppression mechanism MUST amend this spec and include a security rationale.

---

### TL-11: TokenVerifier is a domain port (interface), not a concrete type

**Given** `internal/domain/token.go` (or equivalent domain file)

**When** the Go source is inspected

**Then** `TokenVerifier` MUST be declared as a Go `interface` in the `internal/domain` package

**And** `TokenStatus` MUST be a value type (struct) in `internal/domain`

**And** the concrete Cloudflare adapter implementing `TokenVerifier` MUST reside in
`internal/infra/cloudflare/`, not in `internal/domain/`

---

### TL-12: Token is never persisted to disk

**Given** any execution of `flareout` that reads a token from `CLOUDFLARE_API_TOKEN`

**When** the full execution completes (including any error paths)

**Then** the token value MUST NOT be written to any file, including log files,
cache files, or temporary files

**And** the token MUST be held only in-memory within the process lifetime
