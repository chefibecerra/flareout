package tui

import (
	"log/slog"
	"strconv"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chefibecerra/flareout/internal/domain"
)

// state represents the finite states of the list model.
// stateLoading is a stub reserved for future refresh-in-place (v2+);
// the current constructor always starts in stateLoaded because records are
// pre-fetched by the CLI layer before the Bubbletea program starts (ADR-02).
type state int

const (
	stateLoaded  state = iota
	stateErrored       // reached when an ErrorMsg is received
	stateLoading       // stub — forward-compat for v2 refresh; not used by New()
)

// ErrorMsg carries an error into the model via the Bubbletea message bus.
// Tests and future refresh logic use this to transition the model to stateErrored.
type ErrorMsg struct {
	Err error
}

// Model is the Bubbletea model for the DNS record list screen.
//
// It supports an in-memory multi-select queue: pressing space on a row marks
// the record as a "pending toggle" (a desired flip of the proxied flag).
// The proxied column then renders [P]→[-] (or [-]→[P]) to make the pending
// state visible. Pressing space again on the same row unmarks it. Pressing
// 'a' quits the program with WantsApply()=true so the CLI knows to walk the
// pending map and execute each toggle (with snapshot + audit) AFTER the TUI
// has restored the terminal — this keeps the model itself pure (no I/O,
// no Cloudflare calls, no filesystem writes).
type Model struct {
	state      state
	table      table.Model
	err        error
	records    []domain.Record
	pending    map[string]bool // record ID -> desired proxied state
	wantsApply bool
	width      int
	height     int
	logger     *slog.Logger
}

// New constructs a Model pre-loaded with the given records.
// The model starts in stateLoaded; the table is built immediately from records.
// logger is stored for future structured log writes from within the model.
func New(records []domain.Record, logger *slog.Logger) Model {
	pending := make(map[string]bool)
	return Model{
		state:   stateLoaded,
		table:   buildTable(records, pending),
		records: records,
		pending: pending,
		logger:  logger,
	}
}

// Pending returns a copy of the pending-toggles map: record ID -> desired
// proxied state. The CLI layer reads this after the program quits and walks
// the entries to execute each toggle. Returns an empty map (not nil) if no
// records are marked.
func (m Model) Pending() map[string]bool {
	out := make(map[string]bool, len(m.pending))
	for k, v := range m.pending {
		out[k] = v
	}
	return out
}

// WantsApply reports whether the user pressed 'a' to commit the pending
// toggles. False if the user pressed q/Esc/Ctrl+C (cancel without applying).
func (m Model) WantsApply() bool { return m.wantsApply }

// Records returns the records the model was constructed with. Used by the
// CLI layer to look up record details for each pending toggle.
func (m Model) Records() []domain.Record { return m.records }

// Init satisfies tea.Model. Returns nil because records are pre-fetched.
func (m Model) Init() tea.Cmd { return nil }

// Update handles incoming messages.
//
// Key bindings:
//   - q, Esc, Ctrl+C → tea.Quit (cancel — pending toggles are discarded)
//   - space          → toggle pending state for the row under the cursor
//   - a              → tea.Quit with wantsApply=true (commit pending toggles)
//
// All other messages are forwarded to the embedded table component, which
// owns navigation (arrow keys, j/k, etc).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "a":
			m.wantsApply = true
			return m, tea.Quit
		case " ":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.records) {
				rec := m.records[idx]
				if _, exists := m.pending[rec.ID]; exists {
					delete(m.pending, rec.ID)
				} else {
					// Desired state = the OPPOSITE of current. One press marks
					// "I want to flip this"; pressing again removes the mark.
					m.pending[rec.ID] = !rec.Proxied
				}
				m.table.SetRows(buildRows(m.records, m.pending))
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.height > 4 {
			m.table.SetHeight(m.height - 4)
		}
		return m, nil

	case ErrorMsg:
		m.state = stateErrored
		m.err = msg.Err
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the current model state as a string.
// In stateErrored the error banner is shown; in all other states the table
// is rendered (an empty table renders gracefully when records is nil/empty).
// A footer hint is appended whenever there are pending toggles so the user
// always sees how many marks they have and which key applies them.
func (m Model) View() string {
	if m.state == stateErrored && m.err != nil {
		return "Error: " + m.err.Error() + "\nPress q to quit."
	}
	tbl := m.table.View()
	if len(m.pending) > 0 {
		footer := footerStyle.Render(
			"  " + strconv.Itoa(len(m.pending)) + " pending — press 'a' to apply, 'q' to cancel",
		)
		return tbl + "\n" + footer
	}
	return tbl + "\n" + footerStyle.Render("  space: mark/unmark   a: apply   q: quit")
}

var footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// buildTable constructs a bubbles/table model from a slice of domain records
// and the pending-toggles map. Column widths are fixed; Content is truncated
// at 30 runes before populating rows to avoid overflowing the fixed-width
// column. The Proxied column shows the pending diff when applicable.
func buildTable(records []domain.Record, pending map[string]bool) table.Model {
	cols := []table.Column{
		{Title: "Zone", Width: 25},
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 6},
		{Title: "Content", Width: 32},
		{Title: "Proxied", Width: 9},
		{Title: "TTL", Width: 8},
	}

	rows := buildRows(records, pending)

	initialHeight := len(rows) + 2
	if initialHeight < 4 {
		initialHeight = 4
	}
	if initialHeight > 20 {
		initialHeight = 20
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(initialHeight),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	t.SetStyles(s)
	return t
}

// buildRows is the row factory used both at construction and after every
// space-mark toggle. Pulled out of buildTable so Update() can rebuild rows
// in place via table.Model.SetRows.
func buildRows(records []domain.Record, pending map[string]bool) []table.Row {
	rows := make([]table.Row, 0, len(records))
	for _, r := range records {
		rows = append(rows, table.Row{
			r.ZoneName,
			r.Name,
			r.Type,
			truncateRunes(r.Content, 30),
			proxiedCell(r, pending),
			ttlLabel(r.TTL),
		})
	}
	return rows
}

// proxiedCell renders the Proxied column for a single row. When the record
// is in the pending map it shows the diff arrow [P]->[-]; otherwise it shows
// just the current state.
func proxiedCell(r domain.Record, pending map[string]bool) string {
	cur := proxiedMark(r.Proxied)
	if desired, ok := pending[r.ID]; ok {
		return cur + ">" + proxiedMark(desired)
	}
	return cur
}

func proxiedMark(p bool) string {
	if p {
		return "[P]"
	}
	return "[-]"
}

// truncateRunes truncates s to at most n Unicode runes, appending an ellipsis
// if truncation occurs. Uses rune counting (not byte length) so multibyte
// characters are handled correctly.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

// ttlLabel converts a Cloudflare TTL integer to a display string.
// TTL=1 is Cloudflare's sentinel for "automatic" TTL.
func ttlLabel(ttl int64) string {
	if ttl == 1 {
		return "auto"
	}
	return strconv.FormatInt(ttl, 10)
}
