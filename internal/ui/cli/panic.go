package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chefibecerra/flareout/internal/app"
)

// NewPanicCmd builds the "panic" subcommand. It un-proxies every record in
// a zone in one shot — built for when Cloudflare's proxy is broken and
// the user wants to immediately route around it without togging records
// one at a time. Each per-record write goes through the same safety stack
// as `flareout toggle` (snapshot before, audit after, on every record).
//
// To prevent accidental panic invocations the command requires the user to
// type the zone name verbatim at the confirmation prompt. --yes skips the
// prompt for scripted usage but does NOT skip the snapshot or audit
// writes (those are non-negotiable).
func NewPanicCmd() *cobra.Command {
	var (
		zoneFlag string
		yesFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "panic",
		Short: "Un-proxy every record in a zone (emergency mode)",
		Long: `Un-proxy every currently-proxied record in the named zone in one operation.

This is the emergency lever for "Cloudflare's proxy is broken right now and
I need traffic to bypass it for ALL my records under <zone>". The command
shows the full list of records that would be affected, then requires the
user to type the zone name verbatim before any write happens.

Each per-record write goes through the same safety stack as ` + "`flareout toggle`" + `:
a JSON snapshot is written before the API call, and an audit entry is
appended on every attempt (success or failure). Run ` + "`flareout undo`" + ` after a
panic run to revert records one at a time.

Token scope required: Zone:Edit + DNS:Edit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if zoneFlag == "" {
				return fmt.Errorf("panic: --zone is required")
			}

			appCtx, ok := AppCtxFrom(cmd.Context())
			if !ok {
				return fmt.Errorf("panic: app.Context not in command context")
			}

			snapDir, auditPath, err := stateDirs()
			if err != nil {
				return err
			}

			summary, err := app.PanicPreviewZone(cmd.Context(), appCtx, zoneFlag)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(summary.ProxiedRecords) == 0 {
				_, _ = fmt.Fprintf(out, "no proxied records found in zone %s — nothing to do\n", summary.Zone.Name)
				return nil
			}

			_, _ = fmt.Fprintf(out, "PANIC PREVIEW: %d proxied record(s) in zone %s would be un-proxied:\n",
				len(summary.ProxiedRecords), summary.Zone.Name)
			for _, r := range summary.ProxiedRecords {
				_, _ = fmt.Fprintf(out, "  %-30s %-6s %s\n", r.Name, r.Type, r.Content)
			}

			if !yesFlag {
				if !confirmZoneName(cmd.InOrStdin(), cmd.OutOrStdout(), summary.Zone.Name) {
					_, _ = fmt.Fprintln(out, "aborted")
					return nil
				}
			}

			result, applyErr := app.PanicApplyZone(cmd.Context(), appCtx, summary, snapDir, auditPath)

			var ok2, fail int
			var sawAuthError bool
			for _, r := range result.Results {
				if r.Err != nil {
					fail++
					if is403Auth(r.Err.Error()) {
						sawAuthError = true
					}
					FailLine(out, r.Record.ZoneName, r.Record.Name, r.Err)
				} else {
					ok2++
					_, _ = fmt.Fprintf(out, "  OK   %s/%s now proxied=false\n", r.Record.ZoneName, r.Record.Name)
				}
			}
			_, _ = fmt.Fprintf(out, "PANIC SUMMARY: %d ok, %d failed (out of %d)\n",
				ok2, fail, len(result.Results))

			if sawAuthError {
				PrintError(out, errFailedAuthHint)
			}

			return applyErr
		},
	}

	cmd.Flags().StringVar(&zoneFlag, "zone", "", "Zone name to un-proxy (e.g. example.com). Required.")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "Skip the type-the-zone-name confirmation prompt (scripts).")

	return cmd
}

// confirmZoneName prompts the user to type the zone name verbatim. Returns
// true only when the input matches exactly (after trimming whitespace).
// More resistant than y/N because it forces the user to confirm which
// zone they are panicking on.
func confirmZoneName(in io.Reader, out io.Writer, zone string) bool {
	_, _ = fmt.Fprintf(out, "Type %q to confirm: ", zone)
	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	return strings.TrimSpace(answer) == zone
}
