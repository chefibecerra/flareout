package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/infra/logging"
)

// NewToggleCmd builds the "toggle" subcommand. The command resolves a
// single record by zone+name (with optional --type), computes the proxy
// state diff, and either prints it (dry-run, default) or applies the
// change (when --apply is passed). Apply path writes a snapshot to
// $XDG_STATE_HOME/flareout/snapshots/ and an audit entry to
// $XDG_STATE_HOME/flareout/audit.jsonl before returning.
//
// Selector format: <zone>/<record-name>. The slash splits the two; both
// halves are required. --type disambiguates when multiple records share a
// name (e.g. A and AAAA on www.example.com).
func NewToggleCmd() *cobra.Command {
	var (
		proxiedFlag bool
		applyFlag   bool
		typeFlag    string
	)

	cmd := &cobra.Command{
		Use:   "toggle <zone>/<name>",
		Short: "Toggle the orange-cloud proxy on a single DNS record",
		Long: `Toggle the proxied flag of a single DNS record.

Default mode is dry-run: prints what WOULD change. Pass --apply to write.
Apply path writes a JSON snapshot of the pre-mutation record to disk and
appends a JSONL audit entry before calling Cloudflare's Edit endpoint.

Selector: <zone>/<name>. Use --type to disambiguate (e.g. --type=A) when
the record name has multiple types under the same zone.

Token scope required for --apply: Zone:Edit + DNS:Edit.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zone, name, ok := strings.Cut(args[0], "/")
			if !ok || zone == "" || name == "" {
				return fmt.Errorf("toggle: selector must be <zone>/<name>; got %q", args[0])
			}

			appCtx, ok := AppCtxFrom(cmd.Context())
			if !ok {
				return fmt.Errorf("toggle: app.Context not in command context (PersistentPreRunE was bypassed?)")
			}

			snapDir, auditPath, err := stateDirs()
			if err != nil {
				return err
			}

			result, err := app.ToggleProxy(
				cmd.Context(), appCtx,
				zone, name, typeFlag,
				proxiedFlag, applyFlag,
				snapDir, auditPath,
			)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if result.Diff.IsNoOp() {
				_, _ = fmt.Fprintf(out, "no-op: %s/%s is already proxied=%v\n", result.Diff.Record.ZoneName, result.Diff.Record.Name, result.Diff.ToProxied)
				return nil
			}

			r := result.Diff.Record
			_, _ = fmt.Fprintf(out, "%s %s/%s (%s, %s) proxied: %v -> %v\n",
				modeLabel(applyFlag),
				r.ZoneName, r.Name, r.Type, r.Content,
				result.Diff.FromProxied, result.Diff.ToProxied,
			)
			if applyFlag {
				_, _ = fmt.Fprintf(out, "  snapshot: %s\n", result.SnapshotPath)
				_, _ = fmt.Fprintf(out, "  audit:    %s\n", auditPath)
			} else {
				_, _ = fmt.Fprintln(out, "  (dry-run; pass --apply to write)")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&proxiedFlag, "proxied", false, "Target proxied state (true = orange cloud on, false = off)")
	cmd.Flags().BoolVar(&applyFlag, "apply", false, "Actually apply the change. Without this flag, the command is a dry-run.")
	cmd.Flags().StringVar(&typeFlag, "type", "", "Record type filter (e.g. A, AAAA, CNAME). Required when name has multiple types.")

	return cmd
}

// stateDirs returns (snapshotDir, auditPath) under the same XDG state dir
// already used by SwapToFileJSON. Resolves once per invocation.
func stateDirs() (string, string, error) {
	logPath, err := logging.StateLogPath()
	if err != nil {
		return "", "", fmt.Errorf("toggle: state dir: %w", err)
	}
	stateDir := filepath.Dir(logPath) // .../flareout/
	return filepath.Join(stateDir, "snapshots"), filepath.Join(stateDir, "audit.jsonl"), nil
}

func modeLabel(apply bool) string {
	if apply {
		return "APPLIED:"
	}
	return "WOULD CHANGE:"
}
