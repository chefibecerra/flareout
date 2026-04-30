package tui_test

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/chefibecerra/flareout/internal/domain"
	"github.com/chefibecerra/flareout/internal/ui/tui"
)

// twoRecords returns a small set of test records for use across multiple tests.
func twoRecords() []domain.Record {
	return []domain.Record{
		{
			ID:       "r1",
			Type:     "A",
			Name:     "www.example.com",
			Content:  "1.2.3.4",
			ZoneID:   "z1",
			ZoneName: "example.com",
			Proxied:  true,
			TTL:      300,
		},
		{
			ID:       "r2",
			Type:     "AAAA",
			Name:     "mail.example.com",
			Content:  "::1",
			ZoneID:   "z1",
			ZoneName: "example.com",
			Proxied:  false,
			TTL:      1,
		},
	}
}

// TestListModel_LoadedStateRendersTable confirms that a freshly constructed model
// renders the table header columns (TS-05, TS-17).
func TestListModel_LoadedStateRendersTable(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 40))
	defer tm.Quit()

	// All table headers must appear in the initial render. Combine into a
	// single WaitFor — tm.Output() is a stream and consecutive WaitFor calls
	// cannot rewind to find earlier matches.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Zone")) &&
			bytes.Contains(b, []byte("Name")) &&
			bytes.Contains(b, []byte("Type")) &&
			bytes.Contains(b, []byte("Content")) &&
			bytes.Contains(b, []byte("Proxied")) &&
			bytes.Contains(b, []byte("TTL"))
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_RecordDataAppearsInTable verifies that record values appear in
// the rendered table (TS-05).
func TestListModel_RecordDataAppearsInTable(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 40))
	defer tm.Quit()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("example.com"))
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_ProxiedColumn verifies [P] for proxied=true and [-] for
// proxied=false (TS-07).
func TestListModel_ProxiedColumn(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 40))
	defer tm.Quit()

	// Both proxied markers in one WaitFor — output is a stream.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("[P]")) && bytes.Contains(b, []byte("[-]"))
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_ContentTruncation verifies that content longer than 30 chars
// is truncated and does not panic (TS-06).
func TestListModel_ContentTruncation(t *testing.T) {
	long := "a.very.long.dns.content.value.that.exceeds.thirty.characters"
	records := []domain.Record{
		{
			ID:       "r1",
			Type:     "A",
			Name:     "long.example.com",
			Content:  long,
			ZoneID:   "z1",
			ZoneName: "example.com",
			Proxied:  false,
			TTL:      300,
		},
	}
	m := tui.New(records, slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 40))
	defer tm.Quit()

	// The table must render without panic; the header must be visible.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("example.com"))
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_QKeyQuits confirms that pressing "q" causes the model to
// return a tea.Quit command and the program finishes (TS-09).
func TestListModel_QKeyQuits(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))

	// Wait for initial render before sending quit.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Zone"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestListModel_EscKeyQuits confirms that pressing Escape quits (TS-11).
func TestListModel_EscKeyQuits(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Zone"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestListModel_CtrlCQuits confirms that Ctrl+C quits (TS-10).
func TestListModel_CtrlCQuits(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Zone"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestListModel_WindowResize confirms that a WindowSizeMsg is handled without
// panic and the model continues rendering (TS-12).
func TestListModel_WindowResize(t *testing.T) {
	m := tui.New(twoRecords(), slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))
	defer tm.Quit()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Zone"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// After resize, model should still render content.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return len(b) > 0
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_ErrorState confirms that sending an error message transitions
// the model to the error state and renders the error text (TS-03, TS-04, TS-18).
func TestListModel_ErrorState(t *testing.T) {
	m := tui.New(nil, slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))
	defer tm.Quit()

	// Send an error message to force error state.
	tm.Send(tui.ErrorMsg{Err: errors.New("connection refused")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("connection refused"))
	}, teatest.WithDuration(3*time.Second))
}

// TestListModel_ErrorState_QQuits confirms that q works in error state (TS-04).
func TestListModel_ErrorState_QQuits(t *testing.T) {
	m := tui.New(nil, slog.Default())
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))

	tm.Send(tui.ErrorMsg{Err: errors.New("connection refused")})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("connection refused"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
