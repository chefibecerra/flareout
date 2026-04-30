package cli

import (
	"context"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/infra/logging"
	"github.com/chefibecerra/flareout/internal/ui/tui"
)

// runTUI launches the Bubbletea TUI list screen.
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
func runTUI(ctx context.Context, records []domain.Record, logger *slog.Logger) error {
	logPath, err := logging.StateLogPath()
	if err != nil {
		return err
	}
	restore, err := logging.SwapToFileJSON(logPath)
	if err != nil {
		return err
	}
	defer restore()

	m := tui.New(records, logger)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	return err
}
