package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// SNMP version options for cycling in the picker form.
var pickerSNMPVersions = []string{"2c", "1", "3"}

// Auth protocol options for cycling in the picker form.
var pickerAuthProtocols = []string{"None", "MD5", "SHA", "SHA256", "SHA512"}

// Privacy protocol options for cycling in the picker form.
var pickerPrivProtocols = []string{"None", "DES", "AES128", "AES192", "AES256"}

// Form field indices for the picker form.
const (
	pFieldName     = 0
	pFieldVersion  = 1
	pFieldComm     = 2
	pFieldUser     = 3
	pFieldAuthProt = 4
	pFieldAuthPass = 5
	pFieldPrivProt = 6
	pFieldPrivPass = 7
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
	selectedName string

	// Form mode state (reserved for Task 2)
	formFields  []textinput.Model
	formFocus   int
	formVersion string // current SNMP version
	formAuth    string // current auth protocol
	formPriv    string // current priv protocol
	formErr     string
}

// SelectedName returns the name of the identity selected by the user.
func (m IdentityPickerModel) SelectedName() string { return m.selectedName }

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
	sort.Slice(sums, func(i, j int) bool { return sums[i].Name < sums[j].Name })
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
	case pickerModeForm:
		return m.updateForm(msg)
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
		m.selectedName = ""
		return m, nil, PickerSelected
	}

	// Last row: + New Identity — transition to form mode.
	if m.cursor == total-1 {
		m.initForm()
		return m, nil, PickerNone
	}

	// Rows 1..n: existing identities
	idx := m.cursor - 1
	if idx >= 0 && idx < len(m.summaries) {
		m.selectedName = m.summaries[idx].Name
		return m, nil, PickerSelected
	}

	return m, nil, PickerNone
}

// View renders the picker as a centered modal overlay.
func (m IdentityPickerModel) View() string {
	if m.mode == pickerModeForm {
		return m.viewForm()
	}
	// Modal width: responsive, capped between min 34 (enough for content) and
	// max 56 (identity detail strings like "v3  user:longname  SHA256/AES256"
	// fit comfortably; wider than SwitcherView which has shorter entries).
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
	lines = append(lines, m.renderRow("(none)", "", m.cursor == 0))

	// Rows 1..n: existing identities
	for i, s := range m.summaries {
		label := m.formatSummary(s)
		lines = append(lines, m.renderRow(s.Name, label, m.cursor == i+1))
	}

	// Last row: + New Identity
	newIdx := m.totalItems() - 1
	lines = append(lines, m.renderNewEntry(newIdx))

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
func (m IdentityPickerModel) renderRow(name, detail string, selected bool) string {
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
func (m IdentityPickerModel) renderNewEntry(idx int) string {
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

// padPickerLabel right-pads s with spaces to the given width.
func padPickerLabel(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// --- Form mode ---

// initForm initializes all form fields and transitions to pickerModeForm.
func (m *IdentityPickerModel) initForm() {
	m.mode = pickerModeForm
	m.formErr = ""
	m.formVersion = "2c"
	m.formAuth = "None"
	m.formPriv = "None"

	m.formFields = make([]textinput.Model, 8)

	m.formFields[pFieldName] = textinput.New()
	m.formFields[pFieldName].Placeholder = "identity name"
	m.formFields[pFieldName].CharLimit = 64
	m.formFields[pFieldName].Width = 30

	m.formFields[pFieldVersion] = textinput.New()
	m.formFields[pFieldVersion].Placeholder = "version"
	m.formFields[pFieldVersion].CharLimit = 3
	m.formFields[pFieldVersion].Width = 30
	m.formFields[pFieldVersion].SetValue("2c")

	m.formFields[pFieldComm] = textinput.New()
	m.formFields[pFieldComm].Placeholder = "community string"
	m.formFields[pFieldComm].CharLimit = 128
	m.formFields[pFieldComm].Width = 30
	m.formFields[pFieldComm].EchoMode = textinput.EchoPassword

	m.formFields[pFieldUser] = textinput.New()
	m.formFields[pFieldUser].Placeholder = "SNMPv3 username"
	m.formFields[pFieldUser].CharLimit = 64
	m.formFields[pFieldUser].Width = 30

	m.formFields[pFieldAuthProt] = textinput.New()
	m.formFields[pFieldAuthProt].Placeholder = "auth protocol"
	m.formFields[pFieldAuthProt].CharLimit = 8
	m.formFields[pFieldAuthProt].Width = 30

	m.formFields[pFieldAuthPass] = textinput.New()
	m.formFields[pFieldAuthPass].Placeholder = "auth password"
	m.formFields[pFieldAuthPass].CharLimit = 128
	m.formFields[pFieldAuthPass].Width = 30
	m.formFields[pFieldAuthPass].EchoMode = textinput.EchoPassword

	m.formFields[pFieldPrivProt] = textinput.New()
	m.formFields[pFieldPrivProt].Placeholder = "priv protocol"
	m.formFields[pFieldPrivProt].CharLimit = 8
	m.formFields[pFieldPrivProt].Width = 30

	m.formFields[pFieldPrivPass] = textinput.New()
	m.formFields[pFieldPrivPass].Placeholder = "priv password"
	m.formFields[pFieldPrivPass].CharLimit = 128
	m.formFields[pFieldPrivPass].Width = 30
	m.formFields[pFieldPrivPass].EchoMode = textinput.EchoPassword

	m.formFocus = 0
	m.formFields[pFieldName].Focus()
}

// visibleFormFields returns the indices of fields that should be shown for the
// current SNMP version and protocol selections.
func (m IdentityPickerModel) visibleFormFields() []int {
	fields := []int{pFieldName, pFieldVersion}
	if m.formVersion == "1" || m.formVersion == "2c" {
		fields = append(fields, pFieldComm)
	} else {
		fields = append(fields, pFieldUser, pFieldAuthProt)
		if m.formAuth != "None" {
			fields = append(fields, pFieldAuthPass)
		}
		fields = append(fields, pFieldPrivProt)
		if m.formPriv != "None" {
			fields = append(fields, pFieldPrivPass)
		}
	}
	return fields
}

// focusFormField sets focus to the field at visible index visIdx, clamping to
// valid range and skipping focus on cycle-only fields.
func (m *IdentityPickerModel) focusFormField(visIdx int) {
	visible := m.visibleFormFields()
	if visIdx < 0 {
		visIdx = 0
	}
	if visIdx >= len(visible) {
		visIdx = len(visible) - 1
	}
	m.formFocus = visIdx

	for i := range m.formFields {
		m.formFields[i].Blur()
	}

	fieldIdx := visible[visIdx]
	if fieldIdx != pFieldVersion && fieldIdx != pFieldAuthProt && fieldIdx != pFieldPrivProt {
		m.formFields[fieldIdx].Focus()
	}
}

// cycleVersion cycles the SNMP version forward or backward.
func (m *IdentityPickerModel) cycleVersion(forward bool) {
	idx := 0
	for i, v := range pickerSNMPVersions {
		if v == m.formVersion {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerSNMPVersions)
	} else {
		idx = (idx - 1 + len(pickerSNMPVersions)) % len(pickerSNMPVersions)
	}
	m.formVersion = pickerSNMPVersions[idx]
	m.formFields[pFieldVersion].SetValue(m.formVersion)
}

// cycleAuth cycles the auth protocol forward or backward.
func (m *IdentityPickerModel) cycleAuth(forward bool) {
	idx := 0
	for i, p := range pickerAuthProtocols {
		if p == m.formAuth {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerAuthProtocols)
	} else {
		idx = (idx - 1 + len(pickerAuthProtocols)) % len(pickerAuthProtocols)
	}
	m.formAuth = pickerAuthProtocols[idx]
	m.formFields[pFieldAuthProt].SetValue(m.formAuth)
}

// cyclePriv cycles the priv protocol forward or backward.
func (m *IdentityPickerModel) cyclePriv(forward bool) {
	idx := 0
	for i, p := range pickerPrivProtocols {
		if p == m.formPriv {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerPrivProtocols)
	} else {
		idx = (idx - 1 + len(pickerPrivProtocols)) % len(pickerPrivProtocols)
	}
	m.formPriv = pickerPrivProtocols[idx]
	m.formFields[pFieldPrivProt].SetValue(m.formPriv)
}

// formFieldLabel returns the display label for the given field index.
func (m IdentityPickerModel) formFieldLabel(idx int) string {
	switch idx {
	case pFieldName:
		return "Name"
	case pFieldVersion:
		return "SNMP Version"
	case pFieldComm:
		return "Community"
	case pFieldUser:
		return "Username"
	case pFieldAuthProt:
		return "Auth Protocol"
	case pFieldAuthPass:
		return "Auth Password"
	case pFieldPrivProt:
		return "Priv Protocol"
	case pFieldPrivPass:
		return "Priv Password"
	default:
		return "Unknown"
	}
}

// updateForm handles key input while the picker is in form mode.
func (m IdentityPickerModel) updateForm(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visible := m.visibleFormFields()
		currentField := pFieldName
		if m.formFocus >= 0 && m.formFocus < len(visible) {
			currentField = visible[m.formFocus]
		}

		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			// Return to list mode without saving.
			m.mode = pickerModeList
			m.formErr = ""
			return m, nil, PickerNone

		case msg.String() == "tab":
			m.focusFormField(m.formFocus + 1)
			return m, nil, PickerNone

		case msg.String() == "shift+tab":
			m.focusFormField(m.formFocus - 1)
			return m, nil, PickerNone

		case msg.String() == "enter":
			if currentField == pFieldVersion {
				m.cycleVersion(true)
				return m, nil, PickerNone
			}
			if currentField == pFieldAuthProt {
				m.cycleAuth(true)
				return m, nil, PickerNone
			}
			if currentField == pFieldPrivProt {
				m.cyclePriv(true)
				return m, nil, PickerNone
			}
			// On the last visible field, enter saves.
			if m.formFocus == len(visible)-1 {
				return m.saveForm()
			}
			// Otherwise advance to next field.
			m.focusFormField(m.formFocus + 1)
			return m, nil, PickerNone

		case msg.String() == " ":
			if currentField == pFieldVersion {
				m.cycleVersion(true)
				return m, nil, PickerNone
			}
			if currentField == pFieldAuthProt {
				m.cycleAuth(true)
				return m, nil, PickerNone
			}
			if currentField == pFieldPrivProt {
				m.cyclePriv(true)
				return m, nil, PickerNone
			}
			// Pass space through to text input.
			fallthrough

		default:
			// Skip key input for cycle-only fields.
			if currentField == pFieldVersion || currentField == pFieldAuthProt || currentField == pFieldPrivProt {
				return m, nil, PickerNone
			}
			if m.formFocus >= 0 && m.formFocus < len(visible) {
				fieldIdx := visible[m.formFocus]
				var cmd tea.Cmd
				m.formFields[fieldIdx], cmd = m.formFields[fieldIdx].Update(msg)
				return m, cmd, PickerNone
			}
		}
	}
	return m, nil, PickerNone
}

// saveForm validates the form, calls Provider.Add(), auto-selects the new
// identity, and returns PickerSelected.
func (m IdentityPickerModel) saveForm() (IdentityPickerModel, tea.Cmd, PickerAction) {
	name := strings.TrimSpace(m.formFields[pFieldName].Value())
	if name == "" {
		m.formErr = "Name is required"
		return m, nil, PickerNone
	}

	id := identity.Identity{
		Name:    name,
		Version: m.formVersion,
	}

	if m.formVersion == "1" || m.formVersion == "2c" {
		id.Community = m.formFields[pFieldComm].Value()
	} else {
		id.Username = m.formFields[pFieldUser].Value()
		if m.formAuth != "None" {
			id.AuthProto = m.formAuth
			id.AuthPass = m.formFields[pFieldAuthPass].Value()
		}
		if m.formPriv != "None" {
			id.PrivProto = m.formPriv
			id.PrivPass = m.formFields[pFieldPrivPass].Value()
		}
	}

	if m.provider == nil {
		m.formErr = "No identity store available"
		return m, nil, PickerNone
	}

	if err := m.provider.Add(id); err != nil {
		m.formErr = fmt.Sprintf("Save failed: %v", err)
		return m, nil, PickerNone
	}

	// Reload summaries so the list is up to date.
	m.loadSummaries()
	m.mode = pickerModeList
	m.formErr = ""
	m.selectedName = name
	return m, nil, PickerSelected
}

// viewForm renders the creation form as a centered modal overlay.
func (m IdentityPickerModel) viewForm() string {
	// Use the same responsive width logic as viewList.
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
	innerWidth := modalWidth - 6

	labelStyle := m.sty.FormLabel
	activeLabel := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
	cycleHint := lipgloss.NewStyle().Foreground(m.theme.Base04)
	errStyle := lipgloss.NewStyle().Foreground(m.theme.Base08)
	helpStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)
	helpKeyStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)

	var lines []string

	if m.formErr != "" {
		lines = append(lines, errStyle.Render(m.formErr))
		lines = append(lines, "")
	}

	visible := m.visibleFormFields()
	for vi, fieldIdx := range visible {
		isFocused := vi == m.formFocus
		label := m.formFieldLabel(fieldIdx)

		lbl := labelStyle
		if isFocused {
			lbl = activeLabel
		}
		labelText := lbl.Render(padPickerLabel(label+":", 18))

		indicator := "  "
		if isFocused {
			indicatorStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
			indicator = indicatorStyle.Render("> ")
		}

		switch fieldIdx {
		case pFieldVersion:
			valStyle := lipgloss.NewStyle().Foreground(m.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(m.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			lines = append(lines, fmt.Sprintf("%s%s%s%s", indicator, labelText, valStyle.Render(m.formVersion), hint))

		case pFieldAuthProt:
			valStyle := lipgloss.NewStyle().Foreground(m.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(m.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			lines = append(lines, fmt.Sprintf("%s%s%s%s", indicator, labelText, valStyle.Render(m.formAuth), hint))

		case pFieldPrivProt:
			valStyle := lipgloss.NewStyle().Foreground(m.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(m.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			lines = append(lines, fmt.Sprintf("%s%s%s%s", indicator, labelText, valStyle.Render(m.formPriv), hint))

		default:
			lines = append(lines, fmt.Sprintf("%s%s%s", indicator, labelText, m.formFields[fieldIdx].View()))
		}
	}

	help := fmt.Sprintf(
		"%s/%s navigate  %s save  %s cancel",
		helpKeyStyle.Render("tab"),
		helpKeyStyle.Render("shift+tab"),
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("esc"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(lines, "\n"),
		"",
		helpStyle.Render(help),
	)

	noTopBorder := m.sty.ModalBorder.BorderTop(false)
	modalBody := noTopBorder.Width(innerWidth).Render(content)

	borderFg := lipgloss.NewStyle().Foreground(m.theme.Base0D).Background(m.theme.Base00)
	titleText := " New Identity "
	titleRendered := m.sty.ModalTitle.Render(titleText)

	fullWidth := lipgloss.Width(modalBody)
	rightDashes := fullWidth - 2 - 1 - len(titleText)
	if rightDashes < 0 {
		rightDashes = 0
	}
	topBorder := borderFg.Render("\u256d\u2500") + titleRendered + borderFg.Render(strings.Repeat("\u2500", rightDashes)+"\u256e")

	modal := topBorder + "\n" + modalBody

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}
