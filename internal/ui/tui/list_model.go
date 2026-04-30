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
// It starts in stateLoaded when constructed via New() with pre-fetched records.
type Model struct {
	state   state
	table   table.Model
	err     error
	records []domain.Record
	width   int
	height  int
	logger  *slog.Logger
}

// New constructs a Model pre-loaded with the given records.
// The model starts in stateLoaded; the table is built immediately from records.
// logger is stored for future structured log writes from within the model.
func New(records []domain.Record, logger *slog.Logger) Model {
	return Model{
		state:   stateLoaded,
		table:   buildTable(records),
		records: records,
		logger:  logger,
	}
}

// Init satisfies tea.Model. Returns nil because records are pre-fetched.
func (m Model) Init() tea.Cmd { return nil }

// Update handles incoming messages.
//
// Key bindings:
//   - q, Esc, Ctrl+C → tea.Quit (from any state)
//
// Other messages are forwarded to the embedded table component.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
		if msg.String() == "q" {
			return m, tea.Quit
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
func (m Model) View() string {
	if m.state == stateErrored && m.err != nil {
		return "Error: " + m.err.Error() + "\nPress q to quit."
	}
	return m.table.View()
}

// buildTable constructs a bubbles/table model from a slice of domain records.
// Column widths are fixed; Content is truncated at 30 runes before populating
// rows to avoid overflowing the fixed-width column.
func buildTable(records []domain.Record) table.Model {
	cols := []table.Column{
		{Title: "Zone", Width: 25},
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 6},
		{Title: "Content", Width: 32},
		{Title: "Proxied", Width: 8},
		{Title: "TTL", Width: 8},
	}

	rows := make([]table.Row, 0, len(records))
	for _, r := range records {
		proxied := "[-]"
		if r.Proxied {
			proxied = "[P]"
		}
		rows = append(rows, table.Row{
			r.ZoneName,
			r.Name,
			r.Type,
			truncateRunes(r.Content, 30),
			proxied,
			ttlLabel(r.TTL),
		})
	}

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
