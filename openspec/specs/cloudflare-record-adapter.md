# Spec: Cloudflare Record Adapter

**Capability**: Cloudflare Record Adapter (infra layer)
**Change**: flareout-list
**Phase**: sdd-spec
**Date**: 2026-04-30

---

## Overview

Governs the `cloudflare.RecordLister` struct in `internal/infra/cloudflare/record_lister.go`.
This adapter is the sole file in the repository permitted to import the
`github.com/cloudflare/cloudflare-go/v4` SDK for DNS record access. It satisfies the
`domain.RecordLister` port and maps SDK types to domain types at the boundary.

The constructor follows the same functional-option pattern as the existing
`cloudflare.NewTokenVerifier` adapter. Tests use `httptest.Server` + `WithBaseURL`,
the same pattern established in the foundation.

---

## Scenarios

### CA-01: Constructor signature mirrors NewTokenVerifier pattern

**Given** `internal/infra/cloudflare/record_lister.go` after the change is applied

**When** the exported constructor is inspected

**Then** a function `NewRecordLister(token string, opts ...Option) (*RecordLister, error)` MUST
be exported from the `cloudflare` package

**And** `Option` MUST be the same functional-option type already used by `NewTokenVerifier`

**And** `WithBaseURL(u string) Option` MUST be usable with `NewRecordLister` (not just `NewTokenVerifier`)

**And** the constructor MUST return an error if the underlying SDK client cannot be initialized

---

### CA-02: RecordLister implements domain.RecordLister interface

**Given** `internal/infra/cloudflare.RecordLister` exists

**When** the Go compiler resolves the interface assignment

**Then** `*cloudflare.RecordLister` MUST satisfy `domain.RecordLister` at compile time

**And** a compile-time assertion of the form `var _ domain.RecordLister = (*RecordLister)(nil)`
MUST exist in the adapter file or its test file to document this contract

---

### CA-03: ListZones uses ListAutoPaging with no filter parameters

**Given** a test `httptest.Server` stub configured to return a single-page zones response

**When** `RecordLister.ListZones(ctx)` is called

**Then** the adapter MUST call `client.Zones.ListAutoPaging(ctx, zones.ZoneListParams{})` with an
empty `ZoneListParams` struct (no account-scoping filters in v1)

**And** the HTTP request MUST target the `/client/v4/zones` path (or equivalent SDK-resolved path)

**And** the returned `[]domain.Zone` MUST have one entry per zone in the stub response

**And** each `domain.Zone.ID` MUST equal the `id` field in the stub JSON

**And** each `domain.Zone.Name` MUST equal the `name` field in the stub JSON

**And** each `domain.Zone.Status` MUST equal the `status` field in the stub JSON

---

### CA-04: cloudflare.F(zoneID) MUST be used — not a bare string — for RecordListParams.ZoneID

**Given** a test `httptest.Server` stub configured to return records for a known zone ID `"abc123"`

**When** `RecordLister.ListRecords(ctx, "abc123")` is called

**Then** the outbound HTTP request MUST include `zone_id=abc123` (or equivalent query parameter)
confirming the zone ID was transmitted

**And** the adapter source MUST use `dns.RecordListParams{ZoneID: cloudflare.F(zoneID)}` — a bare
string literal MUST NOT be passed as `ZoneID` because `RecordListParams.ZoneID` is `param.Field[string]`

**Notes**: Passing a bare string to `ZoneID` is a compile error in cloudflare-go/v4 because
`param.Field[string]` is not assignable from `string` directly. The spec documents this to
ensure the test asserts the outbound request contains the zone ID, which catches any accidental
workaround.

---

### CA-05: ListRecords maps SDK RecordResponse fields to domain.Record correctly

**Given** a test `httptest.Server` stub that returns one DNS record with:
- `id`: `"rec1"`
- `type`: `"A"`
- `name`: `"www.example.com"`
- `content`: `"1.2.3.4"`
- `zone_id`: `"abc123"`
- `zone_name`: `"example.com"`
- `proxied`: `true`
- `ttl`: `1`

**When** `RecordLister.ListRecords(ctx, "abc123")` is called

**Then** the returned slice MUST contain exactly one `domain.Record`

**And** all fields MUST map 1:1 from the stub response fields

**And** `domain.Record.Proxied` MUST be `true`

---

### CA-06: Proxied nil maps to false at the adapter boundary

**Given** a test `httptest.Server` stub that returns a DNS record where the `proxied` field is
absent or `null` in the JSON response (representing an NS or TXT record that does not support
proxying)

**When** `RecordLister.ListRecords(ctx, zoneID)` processes that response

**Then** the SDK `dns.RecordResponse.Proxied` will be `nil` (`*bool`)

**And** the resulting `domain.Record.Proxied` MUST be `false`

**And** the resulting `domain.Record.Proxied` MUST NOT be `true`

---

### CA-07: Context cancellation mid-iteration stops ListAutoPaging and returns ctx.Err()

**Given** a test `httptest.Server` that serves a multi-page zones (or records) response
**And** the test cancels the context after the first page is received but before the second page
is fetched

**When** `RecordLister.ListZones(ctx)` or `RecordLister.ListRecords(ctx, zoneID)` is in progress

**Then** the pager loop MUST stop iterating

**And** the method MUST return a non-nil error

**And** the returned error MUST satisfy `errors.Is(err, context.Canceled)`

**And** the method MUST NOT hang or block after the context is cancelled

---

### CA-08: Network error returns a wrapped error (not a panic)

**Given** the test `httptest.Server` is stopped before any request is made
(simulating a network error)

**When** `RecordLister.ListZones(ctx)` is called

**Then** the method MUST return a non-nil error

**And** the method MUST NOT panic

**And** the error MUST be suitable for display to the user (non-empty message)

---

### CA-09: Adapter does not import internal/app or internal/ui

**Given** `internal/infra/cloudflare/record_lister.go` after the change is applied

**When** all import statements in that file are inspected

**Then** no import MUST reference `internal/app` or `internal/ui`

**And** the only internal import permitted is `internal/domain` (to return domain types)

---

### CA-10: Multi-zone multi-page record listing returns all records

**Given** a test `httptest.Server` stub configured to return:
- 2 zones
- Zone 1 with 2 pages of records (3 records total)
- Zone 2 with 1 page of records (2 records total)

**When** `RecordLister.ListRecords` is called for each zone

**Then** the call for zone 1 MUST return exactly 3 records

**And** the call for zone 2 MUST return exactly 2 records

**And** no records from zone 1 MUST appear in the zone 2 result and vice versa

---

### CA-11: Authorization header is set from the token

**Given** a `RecordLister` constructed with token `"test-token-abc"`

**When** `ListZones(ctx)` is called against the test server

**Then** the HTTP request MUST include an `Authorization` header with value `Bearer test-token-abc`
(or the SDK-equivalent header format)

**Notes**: This verifies the constructor wires the token into the underlying SDK client correctly.
The exact header name is dictated by cloudflare-go/v4 — the test observes what the SDK sends.
