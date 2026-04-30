package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chefibecerra/flareout/internal/app"
	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/logging"
	"github.com/chefibecerra/flareout/internal/ui/tui"
)

// runTUI launches the Bubbletea TUI list screen and, on exit, optionally
// runs the post-TUI confirmation+apply flow if the user marked any pending
// proxy toggles.
//
// Order of operations is MANDATORY per the Cobra + Bubbletea lifecycle contract
// documented in internal/ui/tui/doc.go:
//  1. logging.StateLogPath() — resolves path (creates dir as side-effect).
//  2. logging.SwapToFileJSON(path) — redirects slog from stderr to file BEFORE
//     tea.NewProgram starts. Any slog write after this goes to the file, not stderr.
//  3. defer restore() — reinstates previous slog default on function exit.
//  4. tea.NewProgram(...).Run() — owns the terminal until the user quits.
//
// tea.WithContext(ctx) is the integration point for signal cancellation:
// when SIGINT arrives and ctx.Done() closes, Bubbletea dispatches tea.Quit
// cleanly so defer restore() runs before process exit (SH-06).
//
// After the TUI quits, if the FinalModel reports WantsApply()=true with
// pending toggles, runTUI prints a summary to stdout, reads y/N from stdin
// to confirm, and walks the pending map invoking app.ApplyToggle for each.
// The slog handler is restored at this point (the alt-screen has exited),
// so any structured warnings emitted during apply land back on stderr.
func runTUI(
	ctx context.Context,
	records []domain.Record,
	appCtx *app.Context,
	in io.Reader,
	out io.Writer,
) error {
	logPath, err := logging.StateLogPath()
	if err != nil {
		return err
	}
	restore, err := logging.SwapToFileJSON(logPath)
	if err != nil {
		return err
	}

	m := tui.New(records, appCtx.Logger)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	finalModel, runErr := p.Run()

	// Restore stderr slog BEFORE the prompt so post-TUI output is unaffected
	// by the file redirect. Defer was the original pattern but we need the
	// restore to happen NOW (before reading stdin and printing the summary).
	restore()

	if runErr != nil {
		return runErr
	}

	final, ok := finalModel.(tui.Model)
	if !ok {
		return nil
	}
	pending := final.Pending()
	if !final.WantsApply() || len(pending) == 0 {
		return nil
	}

	stateDir := filepath.Dir(logPath)
	snapshotDir := filepath.Join(stateDir, "snapshots")
	auditPath := filepath.Join(stateDir, "audit.jsonl")

	return promptAndApplyPending(ctx, appCtx, records, pending, snapshotDir, auditPath, in, out)
}

// promptAndApplyPending shows the user a per-record diff summary, asks for
// y/n confirmation, and (on y) walks the pending map invoking
// app.ApplyToggle for each entry. Per-record success/failure is printed
// inline; the function returns nil unless every record failed.
func promptAndApplyPending(
	ctx context.Context,
	appCtx *app.Context,
	records []domain.Record,
	pending map[string]bool,
	snapshotDir, auditPath string,
	in io.Reader,
	out io.Writer,
) error {
	byID := indexRecords(records)
	plan := buildPlan(byID, pending)

	if len(plan) == 0 {
		_, _ = fmt.Fprintln(out, "no pending toggles resolved to known records")
		return nil
	}

	_, _ = fmt.Fprintf(out, "About to toggle %d record(s):\n", len(plan))
	for _, item := range plan {
		_, _ = fmt.Fprintf(out, "  %-25s %-30s %-6s proxied: %v -> %v\n",
			item.Record.ZoneName, item.Record.Name, item.Record.Type,
			item.Record.Proxied, item.Desired)
	}
	_, _ = fmt.Fprint(out, "Apply? [y/N]: ")

	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		_, _ = fmt.Fprintln(out, "aborted")
		return nil
	}

	var failures int
	var sawAuthError bool
	for _, item := range plan {
		_, applyErr := app.ApplyToggle(ctx, appCtx, item.Record, item.Desired, snapshotDir, auditPath)
		if applyErr != nil {
			failures++
			if is403Auth(applyErr.Error()) {
				sawAuthError = true
			}
			FailLine(out, item.Record.ZoneName, item.Record.Name, applyErr)
			continue
		}
		_, _ = fmt.Fprintf(out, "  OK   %s/%s now proxied=%v\n", item.Record.ZoneName, item.Record.Name, item.Desired)
	}

	if sawAuthError {
		PrintError(out, errFailedAuthHint)
	}

	if failures > 0 && failures == len(plan) {
		return errors.New("toggle: every record failed (see output)")
	}
	return nil
}

// errFailedAuthHint is reused at the end of bulk runs to print the
// classify-able auth-error hint exactly once after a per-record FAIL line
// streak. It is a sentinel error whose message is what classify() matches
// on for the auth-pattern hint.
var errFailedAuthHint = errors.New("403 Authentication error 10000")

// planItem pairs a target record with its desired proxied state.
type planItem struct {
	Record  domain.Record
	Desired bool
}

func indexRecords(records []domain.Record) map[string]domain.Record {
	out := make(map[string]domain.Record, len(records))
	for _, r := range records {
		out[r.ID] = r
	}
	return out
}

// buildPlan resolves the pending map (record ID -> desired proxied) against
// the known records, sorts by zone name then record name (stable / case-
// insensitive) so the user-facing summary matches the table ordering, and
// drops entries whose record ID was not present in the original list.
func buildPlan(byID map[string]domain.Record, pending map[string]bool) []planItem {
	plan := make([]planItem, 0, len(pending))
	for id, desired := range pending {
		rec, ok := byID[id]
		if !ok {
			continue
		}
		plan = append(plan, planItem{Record: rec, Desired: desired})
	}
	sort.SliceStable(plan, func(i, j int) bool {
		zi, zj := strings.ToLower(plan[i].Record.ZoneName), strings.ToLower(plan[j].Record.ZoneName)
		if zi != zj {
			return zi < zj
		}
		return strings.ToLower(plan[i].Record.Name) < strings.ToLower(plan[j].Record.Name)
	})
	return plan
}
