// Package tui hosts Bubbletea models and screens for FlareOut's interactive surface.
//
// This package is intentionally empty in flareout-foundation — concrete TUI screens
// land in flareout-list and onward. The package exists at this stage so that
// directory layout is fixed and downstream changes inherit the canonical Cobra +
// Bubbletea lifecycle pattern documented below.
//
// # Cobra + Bubbletea Lifecycle Contract
//
// A Cobra command launching a TUI MUST follow this pattern:
//
//  1. Compute the log file path (e.g. ~/.flareout/debug.log — final path is
//     decided in flareout-list).
//  2. Call logging.SwapToFileJSON(path) to redirect slog output from stderr to
//     a JSON file. This MUST happen BEFORE tea.NewProgram(model).Run().
//     Failure to swap will cause terminal corruption when any goroutine logs
//     while the TUI is rendering.
//  3. Defer the returned restore() callback to reinstate the previous slog
//     default when the command returns.
//  4. Construct the Bubbletea model with already-resolved dependencies (do NOT
//     let the model reach into infra directly — the model receives ports).
//  5. Call tea.NewProgram(model, tea.WithAltScreen()).Run().
//  6. On error, propagate to Cobra so the command exits non-zero. The restore
//     callback runs first thanks to the defer, so logs return to stderr in
//     time for the error message to appear cleanly.
//
// VIOLATION: Swapping the slog handler AFTER tea.NewProgram().Run() begins is
// undefined behavior. The first goroutine that logs during TUI rendering will
// write into the alt-screen buffer and visibly corrupt the display.
package tui
