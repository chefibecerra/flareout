# Spec: Record Listing

**Capability**: Record Listing (Domain Types, Port Contract, Use Case)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the `domain.Zone` and `domain.Record` types, the `domain.RecordLister` port interface,
and the `app.ListAllRecords` use case. These form the innermost and application layers of the
hexagonal architecture. No Cloudflare SDK symbol, Cobra flag, or Bubbletea type MAY appear in
any file covered by this spec.

All sort orders and nil-normalization behaviors defined here MUST be consistent across every
output mode (JSON and TUI) because both modes receive their data from the same use case.

---

## Scenarios

### RL-01: domain.Zone struct has exactly the required fields

**Given** `internal/domain/record.go` exists after the change is applied

**When** its exported type `Zone` is inspected

**Then** `Zone` MUST have exactly three string fields: `ID`, `Name`, `Status`

**And** `Zone` MUST NOT have any method receivers, embedded types, or non-string fields

**And** the zero value of `Zone` MUST be valid (all empty strings) without panicking

---

### RL-02: domain.Record struct has exactly the required fields

**Given** `internal/domain/record.go` exists after the change is applied

**When** its exported type `Record` is inspected

**Then** `Record` MUST have the following fields with the following types:
- `ID string`
- `Type string`
- `Name string`
- `Content string`
- `ZoneID string`
- `ZoneName string`
- `Proxied bool` (NOT `*bool`)
- `TTL int64`

**And** `Record` MUST NOT expose `*bool` for `Proxied` at the domain boundary

**And** the zero value of `Record` MUST be valid without panicking

---

### RL-03: Proxied nil from SDK MUST become false at the adapter boundary

**Given** the Cloudflare SDK returns a `dns.RecordResponse` where `Proxied` is a `*bool` set to `nil`
(representing record types that do not support proxying, such as NS or TXT)

**When** the infra adapter maps that SDK response to a `domain.Record`

**Then** the resulting `domain.Record.Proxied` field MUST be `false`

**And** the resulting `domain.Record.Proxied` field MUST NOT be `true` unless the SDK `*bool` was non-nil and pointed to `true`

**Notes**: This normalization MUST occur at the adapter boundary, not in the domain or application layer. The domain type carries `bool`, not `*bool`, so no consumer ever needs a nil check.

---

### RL-04: domain.RecordLister port has exactly two methods

**Given** `internal/domain/record.go` after the change is applied

**When** the exported interface `RecordLister` is inspected

**Then** it MUST declare exactly two methods:
- `ListZones(ctx context.Context) ([]Zone, error)`
- `ListRecords(ctx context.Context, zoneID string) ([]Record, error)`

**And** both methods MUST accept a `context.Context` as their first parameter

**And** `RecordLister` MUST NOT import `internal/infra`, `internal/app`, or `internal/ui`

---

### RL-05: app.ListAllRecords fetches all zones then fans out per zone

**Given** an `app.Context` whose `Lister` field is set to a fake `RecordLister`
**And** the fake returns two zones: `zone-a` and `zone-b`

**When** `app.ListAllRecords(ctx, appCtx)` is called

**Then** `ListZones(ctx)` MUST be called exactly once

**And** `ListRecords(ctx, zoneID)` MUST be called exactly once per zone (twice total)

**And** the returned slice MUST contain all records from both zones

---

### RL-06: Sort order is deterministic — zone name case-insensitive then record name case-insensitive

**Given** `app.ListAllRecords` is called and the fake `RecordLister` returns:
- Zone `"Beta.example.com"` with record `"www"`
- Zone `"alpha.example.com"` with records `"mail"` and `"www"`

**When** the returned slice is inspected

**Then** the records MUST appear in this order:
1. `alpha.example.com / mail`
2. `alpha.example.com / www`
3. `Beta.example.com / www`

**And** the sort MUST be stable across multiple calls with the same input

**And** the sort comparison MUST use `strings.EqualFold`-compatible case folding (Unicode case-insensitive, not ASCII-only toLower)

---

### RL-07: Concurrency cap is exactly 5 goroutines

**Given** an `app.Context` whose `Lister` returns 10 zones
**And** each `ListRecords` call blocks until a test-controlled semaphore is released

**When** `app.ListAllRecords` is called

**Then** at most 5 concurrent calls to `ListRecords` MUST be in-flight at any instant

**And** all 10 zones MUST be processed eventually (no deadlock)

---

### RL-08: errgroup aborts all goroutines on first error

**Given** an `app.Context` whose `Lister` returns 6 zones
**And** `ListRecords` for zone 3 returns a non-nil error `"API error"`
**And** `ListRecords` for zones 1, 2, 4, 5, 6 would succeed if called

**When** `app.ListAllRecords(ctx, appCtx)` is called

**Then** the function MUST return a non-nil error

**And** the returned error MUST wrap or include the original `"API error"` message

**And** the function MUST return before all 6 zones are processed (partial abort)

**And** `app.ListAllRecords` MUST NOT return a partial record slice alongside the error (return MUST be `nil, err`)

---

### RL-09: Context cancellation propagates through the use case

**Given** an `app.Context` whose `Lister.ListZones` blocks until a context deadline
**And** the caller provides a context that is cancelled after 10 milliseconds

**When** `app.ListAllRecords(ctx, appCtx)` is called

**Then** the function MUST return within a reasonable time after cancellation (not hang indefinitely)

**And** the returned error MUST satisfy `errors.Is(err, context.Canceled)` or
`errors.Is(err, context.DeadlineExceeded)` depending on the cancellation cause

---

### RL-10: JSON output shape is a flat array with required fields

**Given** `app.ListAllRecords` returns a non-empty `[]domain.Record`

**When** the CLI JSON writer marshals the result and writes it to stdout

**Then** the output MUST be a valid JSON array

**And** each element MUST contain the following keys with the specified types:
- `"zone_id"`: string
- `"zone_name"`: string
- `"id"`: string
- `"type"`: string
- `"name"`: string
- `"content"`: string
- `"proxied"`: boolean (never `null`)
- `"ttl"`: number (integer)

**And** `"proxied"` MUST be `true` or `false` — NEVER `null` or omitted

---

### RL-11: Empty record set returns an empty JSON array (not null)

**Given** the Cloudflare account has no zones, or all zones have no records

**When** `flareout list --json` is executed

**Then** stdout MUST contain `[]` (empty JSON array)

**And** the process MUST exit with code 0

**And** no error MUST be printed to stderr

---

### RL-12: app.Context gains a Lister field

**Given** `internal/app/context.go` after the change is applied

**When** the `Context` struct is inspected

**Then** it MUST have a field `Lister domain.RecordLister`

**And** `wiring.go` MUST be the only file in `internal/app/` that assigns this field
with a concrete `infra` type

---

### RL-13: Records with empty Content do not cause a panic

**Given** the Cloudflare API returns a `dns.RecordResponse` with an empty `Content` string
(as may occur for SRV or CAA record types)

**When** the adapter maps the response and the result passes through `app.ListAllRecords`

**Then** the resulting `domain.Record.Content` MUST be an empty string `""`

**And** neither the use case, the JSON marshaler, nor the TUI MUST panic

**And** the record MUST appear in the output with an empty content field
