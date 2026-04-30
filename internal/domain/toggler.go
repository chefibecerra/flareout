package domain

import "context"

// Diff describes a pending proxy state change for a single DNS record.
// It is the value passed from the application layer to the CLI for human
// rendering before any write happens. Equality is comparable.
type Diff struct {
	Record      Record
	FromProxied bool
	ToProxied   bool
}

// IsNoOp reports whether the diff represents an unchanged proxy state.
// The CLI uses this to short-circuit writes when the user requests a state
// the record is already in.
func (d Diff) IsNoOp() bool {
	return d.FromProxied == d.ToProxied
}

// RecordToggler is the port for mutating the proxied flag of a single DNS
// record. The application layer wraps invocations with snapshot + audit log
// writes — this port intentionally does only the API mutation.
//
// Implementations MUST preserve all non-proxied fields of the record;
// Cloudflare's Edit endpoint is PUT-shaped (replaces the full record), so
// the caller passes the current state as `current` and the implementation
// reconstructs the full body with proxied swapped to the new value.
type RecordToggler interface {
	SetProxied(ctx context.Context, current Record, proxied bool) error
}
