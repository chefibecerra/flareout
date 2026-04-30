// undo.go is the third documented exception to the app-layer-imports-only-domain
// rule. Like toggle.go, it orchestrates snapshot + audit log reads and the
// reuse of ApplyToggle for the actual mutation. The layering test exempts
// this filename explicitly — see internal/app/layering_test.go.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/audit"
	"github.com/chefibecerra/flareout/internal/infra/snapshot"
)

// UndoResult bundles the outcome of an undo invocation.
//
// SourceEntry is the audit entry that was reverted; Diff describes the
// reversion (FromProxied = the post-toggle state we found on Cloudflare,
// ToProxied = the pre-toggle state we restore from snapshot).
type UndoResult struct {
	SourceEntry  audit.Entry
	Diff         domain.Diff
	Applied      bool
	SnapshotPath string
}

// ErrNothingToUndo is returned when the audit log has no applied entry
// available for reversal.
var ErrNothingToUndo = errors.New("undo: no applied toggle found in audit log")

// UndoLast reverses the most recent applied toggle by reading the audit
// log and snapshot, reconstructing the pre-mutation record, and calling
// ApplyToggle to set proxied back to its pre-toggle value.
//
// The undo is itself a toggle, so it writes its own snapshot and audit
// entry — meaning undo is symmetrically undoable. The chain is bounded
// only by what is in the audit log.
func UndoLast(ctx context.Context, appCtx *Context, snapshotDir, auditPath string) (UndoResult, error) {
	last, err := audit.LastApplied(auditPath)
	if err != nil {
		if errors.Is(err, audit.ErrNoAppliedEntry) {
			return UndoResult{}, ErrNothingToUndo
		}
		return UndoResult{}, fmt.Errorf("undo: %w", err)
	}

	if last.SnapshotPath == "" {
		return UndoResult{}, fmt.Errorf("undo: audit entry for %s/%s has no snapshot path", last.Zone, last.Name)
	}

	snap, err := snapshot.Read(last.SnapshotPath)
	if err != nil {
		return UndoResult{}, fmt.Errorf("undo: %w", err)
	}

	// Construct the "current" record by flipping the snapshot's proxied to
	// the post-toggle value (as recorded in the audit entry). We give
	// ApplyToggle the current record + the desired value (the snapshot's).
	current := snap
	current.Proxied = last.AfterProxied

	result, err := ApplyToggle(ctx, appCtx, current, last.BeforeProxied, snapshotDir, auditPath)
	if err != nil {
		return UndoResult{
			SourceEntry: last,
			Diff:        result.Diff,
			Applied:     result.Applied,
		}, err
	}

	return UndoResult{
		SourceEntry:  last,
		Diff:         result.Diff,
		Applied:      result.Applied,
		SnapshotPath: result.SnapshotPath,
	}, nil
}
