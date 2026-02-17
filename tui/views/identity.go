package views

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

// IdentityMode represents the current mode of the identity view.
type IdentityMode int

const (
	// IdentityList shows the table of identity summaries.
	IdentityList IdentityMode = iota
	// IdentityForm shows the add/edit form.
	IdentityForm
)

// SNMP version options for cycling.
var snmpVersions = []string{"2c", "1", "3"}

// Auth protocol options for cycling.
var authProtocols = []string{"None", "MD5", "SHA", "SHA256", "SHA512"}

// Privacy protocol options for cycling.
var privProtocols = []string{"None", "DES", "AES128", "AES192", "AES256"}

// Form field indices.
const (
	fieldName     = 0
	fieldVersion  = 1
	fieldComm     = 2
	fieldUser     = 3
	fieldAuthProt = 4
	fieldAuthPass = 5
	fieldPrivProt = 6
	fieldPrivPass = 7
)

// IdentityView manages the identity list and form screens.
type IdentityView struct {
	theme    styles.Theme
	sty      *styles.Styles
	provider identity.Provider
	mode     IdentityMode

	// List state
	summaries []identity.Summary
	cursor    int
	width     int
	height    int

	// Form state
	editing     bool   // true if editing existing identity
	editName    string // original name when editing
	formFields  []textinput.Model
	formFocus   int
	formVersion string // current SNMP version in the form
	formAuth    string // current auth protocol selection
	formPriv    string // current priv protocol selection
	err         string
}

// NewIdentityView creates a new IdentityView with the given theme and provider.
func NewIdentityView(theme styles.Theme, provider identity.Provider) IdentityView {
	return IdentityView{
		theme:    theme,
		sty:      styles.NewStyles(theme),
		provider: provider,
		mode:     IdentityList,
	}
}

// SetProvider updates the identity provider.
func (v *IdentityView) SetProvider(provider identity.Provider) {
	v.provider = provider
}

// SetSize updates the available dimensions for the view.
func (v *IdentityView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Refresh reloads identity summaries from the provider.
func (v *IdentityView) Refresh() {
	v.err = ""
	if v.provider == nil {
		v.summaries = nil
		return
	}
	sums, err := v.provider.List()
	if err != nil {
		v.err = fmt.Sprintf("Failed to load identities: %v", err)
		v.summaries = nil
		return
	}
	// Sort summaries by name for stable display order.
	sort.Slice(sums, func(i, j int) bool {
		return sums[i].Name < sums[j].Name
	})
	v.summaries = sums
	if v.cursor >= len(v.summaries) {
		v.cursor = len(v.summaries) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

// Update handles key messages and returns the updated view, a command, and
// whether the user wants to go back to the previous screen.
func (v IdentityView) Update(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	switch v.mode {
	case IdentityList:
		return v.updateList(msg)
	case IdentityForm:
		return v.updateForm(msg)
	}
	return v, nil, false
}

// View renders the identity view based on the current mode.
func (v IdentityView) View() string {
	switch v.mode {
	case IdentityForm:
		return v.viewForm()
	default:
		return v.viewList()
	}
}

// --- List mode ---

func (v IdentityView) updateList(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return v, nil, true

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if v.cursor > 0 {
				v.cursor--
			}
			return v, nil, false

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if v.cursor < len(v.summaries)-1 {
				v.cursor++
			}
			return v, nil, false

		case msg.String() == "n":
			v.initForm(nil)
			return v, nil, false

		case msg.String() == "e":
			if len(v.summaries) > 0 && v.provider != nil {
				name := v.summaries[v.cursor].Name
				id, err := v.provider.Get(name)
				if err == nil {
					v.initForm(id)
				} else {
					v.err = fmt.Sprintf("Failed to load identity: %v", err)
				}
			}
			return v, nil, false

		case msg.String() == "d":
			if len(v.summaries) > 0 && v.provider != nil {
				name := v.summaries[v.cursor].Name
				if err := v.provider.Remove(name); err != nil {
					v.err = fmt.Sprintf("Failed to delete: %v", err)
				} else {
					v.err = ""
				}
				v.Refresh()
			}
			return v, nil, false
		}
	}
	return v, nil, false
}

func (v IdentityView) viewList() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Identities") + "\n")
	b.WriteString("\n")

	if v.provider == nil {
		dimStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
		b.WriteString("  " + dimStyle.Render("No identity store loaded.") + "\n")
		b.WriteString("  " + dimStyle.Render("Configure a store password to get started.") + "\n")
		b.WriteString("\n")
		b.WriteString("  " + v.renderListHelp() + "\n")
		return b.String()
	}

	if v.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(v.theme.Base08)
		b.WriteString("  " + errStyle.Render(v.err) + "\n")
		b.WriteString("\n")
	}

	if len(v.summaries) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
		b.WriteString("  " + dimStyle.Render("No identities defined.") + "\n")
		b.WriteString("\n")
		b.WriteString("  " + v.renderListHelp() + "\n")
		return b.String()
	}

	// Column widths
	nameW := 16
	verW := 9
	userW := 12
	authW := 9
	privW := 9

	// Find max name width
	for _, s := range v.summaries {
		if len(s.Name)+2 > nameW {
			nameW = len(s.Name) + 2
		}
	}
	if nameW > 30 {
		nameW = 30
	}

	// Table header
	headerStyle := v.sty.TableHeader
	header := fmt.Sprintf("  %s%s%s%s%s",
		headerStyle.Render(padRight("Name", nameW)),
		headerStyle.Render(padRight("Version", verW)),
		headerStyle.Render(padRight("Username", userW)),
		headerStyle.Render(padRight("Auth", authW)),
		headerStyle.Render(padRight("Priv", privW)),
	)
	b.WriteString(header + "\n")

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(v.theme.Base03)
	totalW := nameW + verW + userW + authW + privW
	b.WriteString("  " + sepStyle.Render(strings.Repeat("-", totalW)) + "\n")

	// Rows
	for i, s := range v.summaries {
		selected := i == v.cursor

		cursor := "  "
		if selected {
			cursor = "> "
		}

		cursorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
		nameStyle := v.sty.IdentityName
		verStyle := v.sty.IdentityVersion
		rowStyle := v.sty.TableRow
		if selected {
			nameStyle = nameStyle.Background(v.theme.Base02)
			verStyle = verStyle.Background(v.theme.Base02)
			rowStyle = v.sty.TableRowSel
		}

		username := s.Username
		if username == "" {
			username = "-"
		}
		authProto := s.AuthProto
		if authProto == "" {
			authProto = "-"
		}
		privProto := s.PrivProto
		if privProto == "" {
			privProto = "-"
		}

		line := fmt.Sprintf("%s%s%s%s%s%s",
			cursorStyle.Render(cursor),
			nameStyle.Render(padRight(truncate(s.Name, nameW-1), nameW)),
			verStyle.Render(padRight(s.Version, verW)),
			rowStyle.Render(padRight(username, userW)),
			rowStyle.Render(padRight(authProto, authW)),
			rowStyle.Render(padRight(privProto, privW)),
		)
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + v.renderListHelp() + "\n")

	return b.String()
}

func (v IdentityView) renderListHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
	return helpStyle.Render(fmt.Sprintf(
		"%s new  %s edit  %s delete  %s back",
		keyStyle.Render("[n]"),
		keyStyle.Render("[e]"),
		keyStyle.Render("[d]"),
		keyStyle.Render("[esc]"),
	))
}

// --- Form mode ---

// initForm prepares the form fields. If id is nil, it is a new identity;
// otherwise fields are populated from the existing identity.
func (v *IdentityView) initForm(id *identity.Identity) {
	v.mode = IdentityForm
	v.err = ""

	if id != nil {
		v.editing = true
		v.editName = id.Name
		v.formVersion = id.Version
		v.formAuth = id.AuthProto
		if v.formAuth == "" {
			v.formAuth = "None"
		}
		v.formPriv = id.PrivProto
		if v.formPriv == "" {
			v.formPriv = "None"
		}
	} else {
		v.editing = false
		v.editName = ""
		v.formVersion = "2c"
		v.formAuth = "None"
		v.formPriv = "None"
	}

	v.formFields = make([]textinput.Model, 8)

	// Name
	v.formFields[fieldName] = textinput.New()
	v.formFields[fieldName].Placeholder = "identity name"
	v.formFields[fieldName].CharLimit = 64
	v.formFields[fieldName].Width = 30

	// Version (display-only, cycled with Enter/Space)
	v.formFields[fieldVersion] = textinput.New()
	v.formFields[fieldVersion].Placeholder = "version"
	v.formFields[fieldVersion].CharLimit = 3
	v.formFields[fieldVersion].Width = 30

	// Community
	v.formFields[fieldComm] = textinput.New()
	v.formFields[fieldComm].Placeholder = "community string"
	v.formFields[fieldComm].CharLimit = 128
	v.formFields[fieldComm].Width = 30
	v.formFields[fieldComm].EchoMode = textinput.EchoPassword

	// Username
	v.formFields[fieldUser] = textinput.New()
	v.formFields[fieldUser].Placeholder = "SNMPv3 username"
	v.formFields[fieldUser].CharLimit = 64
	v.formFields[fieldUser].Width = 30

	// Auth Protocol (display-only, cycled)
	v.formFields[fieldAuthProt] = textinput.New()
	v.formFields[fieldAuthProt].Placeholder = "auth protocol"
	v.formFields[fieldAuthProt].CharLimit = 8
	v.formFields[fieldAuthProt].Width = 30

	// Auth Password
	v.formFields[fieldAuthPass] = textinput.New()
	v.formFields[fieldAuthPass].Placeholder = "auth password"
	v.formFields[fieldAuthPass].CharLimit = 128
	v.formFields[fieldAuthPass].Width = 30
	v.formFields[fieldAuthPass].EchoMode = textinput.EchoPassword

	// Priv Protocol (display-only, cycled)
	v.formFields[fieldPrivProt] = textinput.New()
	v.formFields[fieldPrivProt].Placeholder = "priv protocol"
	v.formFields[fieldPrivProt].CharLimit = 8
	v.formFields[fieldPrivProt].Width = 30

	// Priv Password
	v.formFields[fieldPrivPass] = textinput.New()
	v.formFields[fieldPrivPass].Placeholder = "priv password"
	v.formFields[fieldPrivPass].CharLimit = 128
	v.formFields[fieldPrivPass].Width = 30
	v.formFields[fieldPrivPass].EchoMode = textinput.EchoPassword

	// Populate values when editing
	if id != nil {
		v.formFields[fieldName].SetValue(id.Name)
		v.formFields[fieldVersion].SetValue(id.Version)
		v.formFields[fieldComm].SetValue(id.Community)
		v.formFields[fieldUser].SetValue(id.Username)
		v.formFields[fieldAuthProt].SetValue(id.AuthProto)
		v.formFields[fieldAuthPass].SetValue(id.AuthPass)
		v.formFields[fieldPrivProt].SetValue(id.PrivProto)
		v.formFields[fieldPrivPass].SetValue(id.PrivPass)
	} else {
		v.formFields[fieldVersion].SetValue("2c")
	}

	v.formFocus = fieldName
	v.formFields[fieldName].Focus()
}

// visibleFormFields returns the indices of fields visible for the current version.
func (v IdentityView) visibleFormFields() []int {
	fields := []int{fieldName, fieldVersion}
	if v.formVersion == "1" || v.formVersion == "2c" {
		fields = append(fields, fieldComm)
	} else {
		// v3
		fields = append(fields, fieldUser, fieldAuthProt)
		if v.formAuth != "None" {
			fields = append(fields, fieldAuthPass)
		}
		fields = append(fields, fieldPrivProt)
		if v.formPriv != "None" {
			fields = append(fields, fieldPrivPass)
		}
	}
	return fields
}

// focusField sets focus to the field at the given visible index.
func (v *IdentityView) focusField(visIdx int) {
	visible := v.visibleFormFields()
	if visIdx < 0 {
		visIdx = 0
	}
	if visIdx >= len(visible) {
		visIdx = len(visible) - 1
	}
	v.formFocus = visIdx

	for i := range v.formFields {
		v.formFields[i].Blur()
	}

	fieldIdx := visible[visIdx]
	// Don't focus cycle fields (version, auth proto, priv proto) as text inputs
	if fieldIdx != fieldVersion && fieldIdx != fieldAuthProt && fieldIdx != fieldPrivProt {
		v.formFields[fieldIdx].Focus()
	}
}

// cycleVersion cycles the SNMP version to the next option.
func (v *IdentityView) cycleVersion(forward bool) {
	idx := 0
	for i, ver := range snmpVersions {
		if ver == v.formVersion {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(snmpVersions)
	} else {
		idx = (idx - 1 + len(snmpVersions)) % len(snmpVersions)
	}
	v.formVersion = snmpVersions[idx]
	v.formFields[fieldVersion].SetValue(v.formVersion)
}

// cycleAuth cycles the auth protocol to the next option.
func (v *IdentityView) cycleAuth(forward bool) {
	idx := 0
	for i, p := range authProtocols {
		if p == v.formAuth {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(authProtocols)
	} else {
		idx = (idx - 1 + len(authProtocols)) % len(authProtocols)
	}
	v.formAuth = authProtocols[idx]
	v.formFields[fieldAuthProt].SetValue(v.formAuth)
}

// cyclePriv cycles the privacy protocol to the next option.
func (v *IdentityView) cyclePriv(forward bool) {
	idx := 0
	for i, p := range privProtocols {
		if p == v.formPriv {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(privProtocols)
	} else {
		idx = (idx - 1 + len(privProtocols)) % len(privProtocols)
	}
	v.formPriv = privProtocols[idx]
	v.formFields[fieldPrivProt].SetValue(v.formPriv)
}

func (v IdentityView) updateForm(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visible := v.visibleFormFields()
		currentField := fieldName
		if v.formFocus >= 0 && v.formFocus < len(visible) {
			currentField = visible[v.formFocus]
		}

		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			// Cancel form, return to list
			v.mode = IdentityList
			v.err = ""
			return v, nil, false

		case msg.String() == "enter":
			// If on a cycle field, cycle the value
			if currentField == fieldVersion {
				v.cycleVersion(true)
				return v, nil, false
			}
			if currentField == fieldAuthProt {
				v.cycleAuth(true)
				return v, nil, false
			}
			if currentField == fieldPrivProt {
				v.cyclePriv(true)
				return v, nil, false
			}
			// Otherwise save the form
			return v.saveForm()

		case msg.String() == " ":
			// Space also cycles on cycle fields
			if currentField == fieldVersion {
				v.cycleVersion(true)
				return v, nil, false
			}
			if currentField == fieldAuthProt {
				v.cycleAuth(true)
				return v, nil, false
			}
			if currentField == fieldPrivProt {
				v.cyclePriv(true)
				return v, nil, false
			}
			// For normal text fields, pass through to the textinput
			return v.updateActiveInput(msg)

		case msg.String() == "tab":
			// Move to next field
			v.focusField(v.formFocus + 1)
			return v, nil, false

		case msg.String() == "shift+tab":
			// Move to previous field
			v.focusField(v.formFocus - 1)
			return v, nil, false

		default:
			// Pass to the active text input (skip cycle fields)
			if currentField == fieldVersion || currentField == fieldAuthProt || currentField == fieldPrivProt {
				return v, nil, false
			}
			return v.updateActiveInput(msg)
		}
	}
	return v, nil, false
}

// updateActiveInput passes a message to the currently focused text input.
func (v IdentityView) updateActiveInput(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	visible := v.visibleFormFields()
	if v.formFocus < 0 || v.formFocus >= len(visible) {
		return v, nil, false
	}
	fieldIdx := visible[v.formFocus]
	var cmd tea.Cmd
	v.formFields[fieldIdx], cmd = v.formFields[fieldIdx].Update(msg)
	return v, cmd, false
}

// saveForm validates and saves the identity from form fields.
func (v IdentityView) saveForm() (IdentityView, tea.Cmd, bool) {
	name := strings.TrimSpace(v.formFields[fieldName].Value())
	if name == "" {
		v.err = "Name is required"
		return v, nil, false
	}

	id := identity.Identity{
		Name:    name,
		Version: v.formVersion,
	}

	if v.formVersion == "1" || v.formVersion == "2c" {
		id.Community = v.formFields[fieldComm].Value()
	} else {
		id.Username = v.formFields[fieldUser].Value()
		if v.formAuth != "None" {
			id.AuthProto = v.formAuth
			id.AuthPass = v.formFields[fieldAuthPass].Value()
		}
		if v.formPriv != "None" {
			id.PrivProto = v.formPriv
			id.PrivPass = v.formFields[fieldPrivPass].Value()
		}
	}

	if v.provider == nil {
		v.err = "No identity store available"
		return v, nil, false
	}

	var err error
	if v.editing {
		err = v.provider.Update(v.editName, id)
	} else {
		err = v.provider.Add(id)
	}
	if err != nil {
		v.err = fmt.Sprintf("Save failed: %v", err)
		return v, nil, false
	}

	v.mode = IdentityList
	v.err = ""
	v.Refresh()
	return v, nil, false
}

func (v IdentityView) viewForm() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)

	var b strings.Builder

	b.WriteString("\n")
	if v.editing {
		b.WriteString("  " + titleStyle.Render("Edit Identity") + "\n")
	} else {
		b.WriteString("  " + titleStyle.Render("New Identity") + "\n")
	}
	b.WriteString("\n")

	if v.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(v.theme.Base08)
		b.WriteString("  " + errStyle.Render(v.err) + "\n")
		b.WriteString("\n")
	}

	visible := v.visibleFormFields()

	labelStyle := v.sty.FormLabel
	activeLabel := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)
	cycleHint := lipgloss.NewStyle().
		Foreground(v.theme.Base04)

	for vi, fieldIdx := range visible {
		isFocused := vi == v.formFocus
		label := v.fieldLabel(fieldIdx)

		lbl := labelStyle
		if isFocused {
			lbl = activeLabel
		}

		// Render label (right-padded to 18 chars for alignment)
		labelText := lbl.Render(padRight(label+":", 18))

		switch fieldIdx {
		case fieldVersion:
			// Render as a cycle selector
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			valStyle := lipgloss.NewStyle().Foreground(v.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(v.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			b.WriteString(fmt.Sprintf("  %s%s%s%s\n", indicator, labelText, valStyle.Render(v.formVersion), hint))

		case fieldAuthProt:
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			valStyle := lipgloss.NewStyle().Foreground(v.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(v.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			b.WriteString(fmt.Sprintf("  %s%s%s%s\n", indicator, labelText, valStyle.Render(v.formAuth), hint))

		case fieldPrivProt:
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			valStyle := lipgloss.NewStyle().Foreground(v.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(v.theme.Base02)
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space to cycle)")
			}
			b.WriteString(fmt.Sprintf("  %s%s%s%s\n", indicator, labelText, valStyle.Render(v.formPriv), hint))

		default:
			// Regular text input field
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			b.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, labelText, v.formFields[fieldIdx].View()))
		}
	}

	b.WriteString("\n")
	b.WriteString("  " + v.renderFormHelp() + "\n")

	return b.String()
}

func (v IdentityView) fieldLabel(idx int) string {
	switch idx {
	case fieldName:
		return "Name"
	case fieldVersion:
		return "SNMP Version"
	case fieldComm:
		return "Community"
	case fieldUser:
		return "Username"
	case fieldAuthProt:
		return "Auth Protocol"
	case fieldAuthPass:
		return "Auth Password"
	case fieldPrivProt:
		return "Priv Protocol"
	case fieldPrivPass:
		return "Priv Password"
	default:
		return "Unknown"
	}
}

func (v IdentityView) renderFormHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
	return helpStyle.Render(fmt.Sprintf(
		"%s/%s navigate  %s save  %s cancel",
		keyStyle.Render("[tab]"),
		keyStyle.Render("[shift+tab]"),
		keyStyle.Render("[enter]"),
		keyStyle.Render("[esc]"),
	))
}
