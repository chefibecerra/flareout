package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chefibecerra/flareout/internal/app"
)

// NewUndoCmd builds the "undo" subcommand. It reverts the most recent
// applied toggle by reading the audit log and snapshot, then calls
// ApplyToggle to set proxied back to its pre-toggle value. The undo is
// itself recorded as a new audit entry with its own snapshot — so
// `flareout undo` after `flareout undo` redoes the original change.
//
// No --apply flag here: undo is the explicit, intentional action. If the
// user did not mean to undo they can run undo again to re-apply.
func NewUndoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "Revert the most recent applied toggle",
		Long: `Read the latest applied entry in the audit log, recover the
pre-mutation record from its snapshot, and call Cloudflare's Edit endpoint
to put the proxied flag back where it was. The undo itself produces a new
snapshot and audit entry — so undo is symmetrically undoable.

Token scope required: Zone:Edit + DNS:Edit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx, ok := AppCtxFrom(cmd.Context())
			if !ok {
				return fmt.Errorf("undo: app.Context not in command context")
			}

			snapDir, auditPath, err := stateDirs()
			if err != nil {
				return err
			}

			result, err := app.UndoLast(cmd.Context(), appCtx, snapDir, auditPath)
			if err != nil {
				if errors.Is(err, app.ErrNothingToUndo) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "nothing to undo: audit log has no applied entries")
					return nil
				}
				return err
			}

			out := cmd.OutOrStdout()
			src := result.SourceEntry
			r := result.Diff.Record
			_, _ = fmt.Fprintf(out, "REVERTED: %s/%s (%s) proxied: %v -> %v\n",
				r.ZoneName, r.Name, r.Type,
				result.Diff.FromProxied, result.Diff.ToProxied,
			)
			_, _ = fmt.Fprintf(out, "  source audit entry: %s (applied at %s)\n",
				src.Name, src.Timestamp.Format("2006-01-02T15:04:05Z"))
			if result.SnapshotPath != "" {
				_, _ = fmt.Fprintf(out, "  new snapshot:       %s\n", result.SnapshotPath)
			}
			return nil
		},
	}
}
