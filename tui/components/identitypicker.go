package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// PickerAction describes the result of an IdentityPickerModel update.
type PickerAction int

const (
	// PickerNone means no action; the picker remains open.
	PickerNone PickerAction = iota
	// PickerSelected means the user chose an identity (may be empty string for none).
	PickerSelected
	// PickerCancelled means the user dismissed the picker without a selection.
	PickerCancelled
)

// pickerMode is the internal display mode of the picker.
type pickerMode int

const (
	pickerModeList pickerMode = iota
	pickerModeForm
)

// IdentityPickerModel is a modal overlay for selecting or creating an identity.
// List mode is implemented here; form mode fields are reserved for Task 2.
type IdentityPickerModel struct {
	theme    styles.Theme
	sty      *styles.Styles
	provider identity.Provider

	// Layout
	width  int
	height int

	// List mode state
	mode      pickerMode
	summaries []identity.Summary
	cursor    int // 0 = (none), 1..n = identity, n+1 = + New Identity

	// Selected result (populated on PickerSelected)
	SelectedName string

	// Form mode state (reserved for Task 2)
	formFields  []textinput.Model
	formFocus   int
	formVersion string // current SNMP version
	formAuth    string // current auth protocol
	formPriv    string // current priv protocol
	formEditing bool   // true when editing an existing identity
	formErr     string
}

// NewIdentityPickerModel creates a new IdentityPickerModel ready to display.
// provider may be nil, in which case only the "(none)" entry is shown.
func NewIdentityPickerModel(theme styles.Theme, provider identity.Provider) IdentityPickerModel {
	m := IdentityPickerModel{
		theme:    theme,
		sty:      styles.NewStyles(theme),
		provider: provider,
		mode:     pickerModeList,
	}
	m.loadSummaries()
	return m
}

// SetSize updates the available terminal dimensions for centering the overlay.
func (m *IdentityPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// loadSummaries fetches the identity list from the provider. Safe to call with
// a nil provider.
func (m *IdentityPickerModel) loadSummaries() {
	m.summaries = nil
	if m.provider == nil {
		return
	}
	sums, err := m.provider.List()
	if err != nil {
		return
	}
	m.summaries = sums
}

// totalItems returns the total number of rows shown in list mode:
// 1 (none) + len(summaries) + 1 (+ New Identity).
func (m IdentityPickerModel) totalItems() int {
	return 1 + len(m.summaries) + 1
}

// Update handles key messages and returns the updated model, a command, and the
// resulting PickerAction.
func (m IdentityPickerModel) Update(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch m.mode {
	case pickerModeList:
		return m.updateList(msg)
	}
	return m, nil, PickerNone
}

func (m IdentityPickerModel) updateList(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return m, nil, PickerCancelled

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil, PickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if m.cursor < m.totalItems()-1 {
				m.cursor++
			}
			return m, nil, PickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			return m.confirmSelection()
		}
	}
	return m, nil, PickerNone
}

// confirmSelection interprets the current cursor position and returns the
// appropriate action.
func (m IdentityPickerModel) confirmSelection() (IdentityPickerModel, tea.Cmd, PickerAction) {
	total := m.totalItems()
	if total == 0 {
		return m, nil, PickerNone
	}

	// Row 0: (none)
	if m.cursor == 0 {
		m.SelectedName = ""
		return m, nil, PickerSelected
	}

	// Last row: + New Identity (form mode not yet implemented)
	if m.cursor == total-1 {
		return m, nil, PickerCancelled
	}

	// Rows 1..n: existing identities
	idx := m.cursor - 1
	if idx >= 0 && idx < len(m.summaries) {
		m.SelectedName = m.summaries[idx].Name
		return m, nil, PickerSelected
	}

	return m, nil, PickerNone
}

// View renders the picker as a centered modal overlay.
func (m IdentityPickerModel) View() string {
	// Modal width: responsive, capped between 34 and 56 characters.
	modalWidth := 44
	if m.width > 60 {
		modalWidth = m.width / 2
		if modalWidth > 56 {
			modalWidth = 56
		}
	}
	if modalWidth < 34 {
		modalWidth = 34
	}

	// Inner content width: subtract border (1 each side) + padding (2 each side) = 6.
	innerWidth := modalWidth - 6

	var lines []string

	if m.provider == nil {
		dimStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)
		lines = append(lines, dimStyle.Render("No identity store loaded."))
		lines = append(lines, "")
	}

	// Row 0: (none)
	lines = append(lines, m.renderRow(0, "(none)", "", m.cursor == 0))

	// Rows 1..n: existing identities
	for i, s := range m.summaries {
		label := m.formatSummary(s)
		lines = append(lines, m.renderRow(i+1, s.Name, label, m.cursor == i+1))
	}

	// Last row: + New Identity
	newIdx := m.totalItems() - 1
	lines = append(lines, m.renderNewEntry(newIdx, innerWidth))

	// Help line
	helpStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)
	helpKeyStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
	help := fmt.Sprintf(
		"%s:select  %s:cancel",
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("esc"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(lines, "\n"),
		"",
		helpStyle.Render(help),
	)

	// Modal body without top border (we draw the top border manually with title).
	noTopBorder := m.sty.ModalBorder.BorderTop(false)
	modalBody := noTopBorder.Width(innerWidth).Render(content)

	// Top border with embedded title.
	borderFg := lipgloss.NewStyle().Foreground(m.theme.Base0D).Background(m.theme.Base00)
	titleText := " Select Identity "
	titleRendered := m.sty.ModalTitle.Render(titleText)

	fullWidth := lipgloss.Width(modalBody)
	rightDashes := fullWidth - 2 - 1 - len(titleText) // corners(2) + one dash + title visual width
	if rightDashes < 0 {
		rightDashes = 0
	}
	topBorder := borderFg.Render("\u256d\u2500") + titleRendered + borderFg.Render(strings.Repeat("\u2500", rightDashes)+"\u256e")

	modal := topBorder + "\n" + modalBody

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

// formatSummary returns the display detail string for an identity summary.
// Examples:
//
//	"v2c"
//	"v3  user:bob  SHA/AES128"
func (m IdentityPickerModel) formatSummary(s identity.Summary) string {
	if s.Version != "3" {
		return "v" + s.Version
	}
	detail := "v3"
	if s.Username != "" {
		detail += "  user:" + s.Username
	}
	if s.AuthProto != "" || s.PrivProto != "" {
		proto := s.AuthProto
		if s.PrivProto != "" {
			if proto != "" {
				proto += "/" + s.PrivProto
			} else {
				proto = s.PrivProto
			}
		}
		if proto != "" {
			detail += "  " + proto
		}
	}
	return detail
}

// renderRow renders a single list row with cursor indicator, name, and detail.
func (m IdentityPickerModel) renderRow(idx int, name, detail string, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}

	cursorStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(m.theme.Base05)
	detailStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)

	if selected {
		nameStyle = nameStyle.Foreground(m.theme.Base06).Bold(true)
		detailStyle = detailStyle.Foreground(m.theme.Base05)
	}

	if detail == "" {
		return cursorStyle.Render(cursor) + nameStyle.Render(name)
	}
	return cursorStyle.Render(cursor) + nameStyle.Render(name) + "  " + detailStyle.Render(detail)
}

// renderNewEntry renders the "+ New Identity" action row.
func (m IdentityPickerModel) renderNewEntry(idx int, _ int) string {
	selected := m.cursor == idx
	cursor := "  "
	if selected {
		cursor = "> "
	}

	cursorStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
	newStyle := lipgloss.NewStyle().Foreground(m.theme.Base0B)
	if selected {
		newStyle = newStyle.Bold(true)
	}

	return cursorStyle.Render(cursor) + newStyle.Render("+ New Identity")
}
