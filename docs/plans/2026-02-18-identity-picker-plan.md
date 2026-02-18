# Identity Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace all free-text identity inputs with a reusable modal overlay picker that shows existing identities, a "none" option, and supports inline creation of new identities.

**Architecture:** New `IdentityPickerModel` component in `tui/components/` following the `SwitcherView` modal overlay pattern. Four calling views (builder, editor, settings, add-host) embed the picker and toggle it when identity fields are activated. The picker has two internal modes: list selection and inline identity creation form.

**Tech Stack:** Go, Bubble Tea, lipgloss, bubbles/textinput, identity.Provider

---

### Task 1: Create the IdentityPickerModel — list mode

**Files:**
- Create: `tui/components/identitypicker.go`

**Step 1: Create the picker component with list mode**

Create `tui/components/identitypicker.go` with the list selection mode. The picker is a modal overlay component.

```go
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

// PickerAction describes what the caller should do after a picker update.
type PickerAction int

const (
	PickerNone      PickerAction = iota // no action, keep showing picker
	PickerSelected                       // user picked an identity; call SelectedName()
	PickerCancelled                      // user pressed esc
)

type pickerMode int

const (
	pickerModeList pickerMode = iota
	pickerModeForm
)

// pickerItem represents a single entry in the picker list.
type pickerItem struct {
	name    string // display name; "" for the (none) entry
	display string // pre-rendered display text
	isNone  bool   // true for the (none) entry
	isNew   bool   // true for the "+ New Identity" entry
}

// SNMP version options for cycling.
var pickerSNMPVersions = []string{"2c", "1", "3"}

// Auth protocol options for cycling.
var pickerAuthProtocols = []string{"None", "MD5", "SHA", "SHA256", "SHA512"}

// Privacy protocol options for cycling.
var pickerPrivProtocols = []string{"None", "DES", "AES128", "AES192", "AES256"}

// Form field indices.
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

// IdentityPickerModel is a modal overlay for selecting or creating identities.
type IdentityPickerModel struct {
	theme    styles.Theme
	sty      *styles.Styles
	provider identity.Provider
	mode     pickerMode

	// List state
	items        []pickerItem
	cursor       int
	selectedName string
	width        int
	height       int

	// Form state
	formFields  []textinput.Model
	formFocus   int
	formVersion string
	formAuth    string
	formPriv    string
	err         string
}

// NewIdentityPicker creates a new picker with the given theme and provider.
func NewIdentityPicker(theme styles.Theme, provider identity.Provider) IdentityPickerModel {
	p := IdentityPickerModel{
		theme:    theme,
		sty:      styles.NewStyles(theme),
		provider: provider,
		mode:     pickerModeList,
	}
	p.refreshList()
	return p
}

// SetSize updates the available dimensions for the modal overlay.
func (p *IdentityPickerModel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SelectedName returns the name of the selected identity ("" means none/cleared).
func (p IdentityPickerModel) SelectedName() string {
	return p.selectedName
}

// refreshList rebuilds the item list from the provider.
func (p *IdentityPickerModel) refreshList() {
	p.items = nil

	// Always add (none) first
	p.items = append(p.items, pickerItem{
		name:    "",
		display: "(none)",
		isNone:  true,
	})

	// Add identities from the provider
	if p.provider != nil {
		if sums, err := p.provider.List(); err == nil {
			sort.Slice(sums, func(i, j int) bool {
				return sums[i].Name < sums[j].Name
			})
			for _, s := range sums {
				display := s.Name + "  v" + s.Version
				if s.Version == "3" && s.Username != "" {
					display += " user:" + s.Username
					if s.AuthProto != "" {
						display += " " + s.AuthProto
					}
					if s.PrivProto != "" {
						display += "/" + s.PrivProto
					}
				}
				p.items = append(p.items, pickerItem{
					name:    s.Name,
					display: display,
				})
			}
		}
	}

	// Always add "+ New Identity" last
	p.items = append(p.items, pickerItem{
		name:    "",
		display: "+ New Identity",
		isNew:   true,
	})

	// Clamp cursor
	if p.cursor >= len(p.items) {
		p.cursor = len(p.items) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// Update handles key messages and returns the updated picker, a command, and an action.
func (p IdentityPickerModel) Update(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch p.mode {
	case pickerModeList:
		return p.updateList(msg)
	case pickerModeForm:
		return p.updateForm(msg)
	}
	return p, nil, PickerNone
}

// --- List mode ---

func (p IdentityPickerModel) updateList(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return p, nil, PickerCancelled

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil, PickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if p.cursor < len(p.items)-1 {
				p.cursor++
			}
			return p, nil, PickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			if len(p.items) == 0 {
				return p, nil, PickerNone
			}
			item := p.items[p.cursor]
			if item.isNew {
				p.initForm()
				return p, nil, PickerNone
			}
			p.selectedName = item.name
			return p, nil, PickerSelected
		}
	}
	return p, nil, PickerNone
}

// View renders the picker as a centered modal box.
func (p IdentityPickerModel) View() string {
	switch p.mode {
	case pickerModeForm:
		return p.viewForm()
	default:
		return p.viewList()
	}
}

func (p IdentityPickerModel) viewList() string {
	// Calculate modal dimensions
	modalWidth := 44
	if p.width > 60 {
		modalWidth = p.width / 2
		if modalWidth > 60 {
			modalWidth = 60
		}
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	innerWidth := modalWidth - 6

	var lines []string

	if p.provider == nil {
		dimStyle := lipgloss.NewStyle().Foreground(p.theme.Base04)
		lines = append(lines, dimStyle.Render("No identity store loaded."))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Press [i] to set up a store."))
		// Still show (none) option
		noneItem := p.items[0]
		cursorStr := "> "
		cursorStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
		nameStyle := lipgloss.NewStyle().Foreground(p.theme.Base06).Bold(true)
		lines = append(lines, "")
		lines = append(lines, cursorStyle.Render(cursorStr)+nameStyle.Render(noneItem.display))
	} else {
		for i, item := range p.items {
			line := p.renderListItem(item, i == p.cursor, innerWidth)
			lines = append(lines, line)
		}
	}

	// Help line
	helpStyle := lipgloss.NewStyle().Foreground(p.theme.Base04)
	helpKeyStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
	help := fmt.Sprintf(
		"%s:select  %s:close",
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("esc"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(lines, "\n"),
		"",
		helpStyle.Render(help),
	)

	// Render modal body without top border
	noTopBorder := p.sty.ModalBorder.BorderTop(false)
	modalBody := noTopBorder.Width(innerWidth).Render(content)

	// Build top border with embedded title
	borderFg := lipgloss.NewStyle().Foreground(p.theme.Base0D).Background(p.theme.Base00)
	titleText := " Select Identity "
	titleRendered := p.sty.ModalTitle.Render(titleText)

	fullWidth := lipgloss.Width(modalBody)
	rightDashes := fullWidth - 2 - 1 - len(titleText)
	if rightDashes < 0 {
		rightDashes = 0
	}
	topBorder := borderFg.Render("╭─") + titleRendered + borderFg.Render(strings.Repeat("─", rightDashes)+"╮")

	modal := topBorder + "\n" + modalBody

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, modal)
}

func (p IdentityPickerModel) renderListItem(item pickerItem, selected bool, width int) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}

	cursorStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(p.theme.Base05)
	if selected {
		nameStyle = nameStyle.Foreground(p.theme.Base06).Bold(true)
	}

	if item.isNone {
		dimStyle := lipgloss.NewStyle().Foreground(p.theme.Base04)
		if selected {
			dimStyle = dimStyle.Foreground(p.theme.Base05).Bold(true)
		}
		return cursorStyle.Render(cursor) + dimStyle.Render(item.display)
	}

	if item.isNew {
		newStyle := lipgloss.NewStyle().Foreground(p.theme.Base0B)
		if selected {
			newStyle = newStyle.Bold(true)
		}
		return cursorStyle.Render(cursor) + newStyle.Render(item.display)
	}

	return cursorStyle.Render(cursor) + nameStyle.Render(item.display)
}
```

**Step 2: Build and verify compilation**

Run: `make install`
Expected: Clean compilation with no errors.

**Step 3: Commit**

```
git add tui/components/identitypicker.go
git commit -m "Add IdentityPickerModel component with list selection mode"
```

---

### Task 2: Add the identity creation form mode to the picker

**Files:**
- Modify: `tui/components/identitypicker.go`

**Step 1: Add the form mode methods**

Add `initForm()`, `updateForm()`, `viewForm()`, and supporting helper methods to the picker. These follow the same field pattern as `tui/views/identity.go:493-957` but are self-contained within the picker component.

Add these methods to `identitypicker.go`:

```go
// --- Form mode ---

func (p *IdentityPickerModel) initForm() {
	p.mode = pickerModeForm
	p.err = ""
	p.formVersion = "2c"
	p.formAuth = "None"
	p.formPriv = "None"

	p.formFields = make([]textinput.Model, 8)

	p.formFields[pFieldName] = textinput.New()
	p.formFields[pFieldName].Placeholder = "identity name"
	p.formFields[pFieldName].CharLimit = 64
	p.formFields[pFieldName].Width = 30

	p.formFields[pFieldVersion] = textinput.New()
	p.formFields[pFieldVersion].Placeholder = "version"
	p.formFields[pFieldVersion].CharLimit = 3
	p.formFields[pFieldVersion].Width = 30
	p.formFields[pFieldVersion].SetValue("2c")

	p.formFields[pFieldComm] = textinput.New()
	p.formFields[pFieldComm].Placeholder = "community string"
	p.formFields[pFieldComm].CharLimit = 128
	p.formFields[pFieldComm].Width = 30
	p.formFields[pFieldComm].EchoMode = textinput.EchoPassword

	p.formFields[pFieldUser] = textinput.New()
	p.formFields[pFieldUser].Placeholder = "SNMPv3 username"
	p.formFields[pFieldUser].CharLimit = 64
	p.formFields[pFieldUser].Width = 30

	p.formFields[pFieldAuthProt] = textinput.New()
	p.formFields[pFieldAuthProt].Placeholder = "auth protocol"
	p.formFields[pFieldAuthProt].CharLimit = 8
	p.formFields[pFieldAuthProt].Width = 30

	p.formFields[pFieldAuthPass] = textinput.New()
	p.formFields[pFieldAuthPass].Placeholder = "auth password"
	p.formFields[pFieldAuthPass].CharLimit = 128
	p.formFields[pFieldAuthPass].Width = 30
	p.formFields[pFieldAuthPass].EchoMode = textinput.EchoPassword

	p.formFields[pFieldPrivProt] = textinput.New()
	p.formFields[pFieldPrivProt].Placeholder = "priv protocol"
	p.formFields[pFieldPrivProt].CharLimit = 8
	p.formFields[pFieldPrivProt].Width = 30

	p.formFields[pFieldPrivPass] = textinput.New()
	p.formFields[pFieldPrivPass].Placeholder = "priv password"
	p.formFields[pFieldPrivPass].CharLimit = 128
	p.formFields[pFieldPrivPass].Width = 30
	p.formFields[pFieldPrivPass].EchoMode = textinput.EchoPassword

	p.formFocus = 0
	p.formFields[pFieldName].Focus()
}

func (p IdentityPickerModel) visibleFormFields() []int {
	fields := []int{pFieldName, pFieldVersion}
	if p.formVersion == "1" || p.formVersion == "2c" {
		fields = append(fields, pFieldComm)
	} else {
		fields = append(fields, pFieldUser, pFieldAuthProt)
		if p.formAuth != "None" {
			fields = append(fields, pFieldAuthPass)
		}
		fields = append(fields, pFieldPrivProt)
		if p.formPriv != "None" {
			fields = append(fields, pFieldPrivPass)
		}
	}
	return fields
}

func (p *IdentityPickerModel) focusFormField(visIdx int) {
	visible := p.visibleFormFields()
	if visIdx < 0 {
		visIdx = 0
	}
	if visIdx >= len(visible) {
		visIdx = len(visible) - 1
	}
	p.formFocus = visIdx

	for i := range p.formFields {
		p.formFields[i].Blur()
	}

	fieldIdx := visible[visIdx]
	if fieldIdx != pFieldVersion && fieldIdx != pFieldAuthProt && fieldIdx != pFieldPrivProt {
		p.formFields[fieldIdx].Focus()
	}
}

func (p *IdentityPickerModel) cycleVersion(forward bool) {
	idx := 0
	for i, ver := range pickerSNMPVersions {
		if ver == p.formVersion {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerSNMPVersions)
	} else {
		idx = (idx - 1 + len(pickerSNMPVersions)) % len(pickerSNMPVersions)
	}
	p.formVersion = pickerSNMPVersions[idx]
	p.formFields[pFieldVersion].SetValue(p.formVersion)
}

func (p *IdentityPickerModel) cycleAuth(forward bool) {
	idx := 0
	for i, proto := range pickerAuthProtocols {
		if proto == p.formAuth {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerAuthProtocols)
	} else {
		idx = (idx - 1 + len(pickerAuthProtocols)) % len(pickerAuthProtocols)
	}
	p.formAuth = pickerAuthProtocols[idx]
	p.formFields[pFieldAuthProt].SetValue(p.formAuth)
}

func (p *IdentityPickerModel) cyclePriv(forward bool) {
	idx := 0
	for i, proto := range pickerPrivProtocols {
		if proto == p.formPriv {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(pickerPrivProtocols)
	} else {
		idx = (idx - 1 + len(pickerPrivProtocols)) % len(pickerPrivProtocols)
	}
	p.formPriv = pickerPrivProtocols[idx]
	p.formFields[pFieldPrivProt].SetValue(p.formPriv)
}

func (p IdentityPickerModel) updateForm(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		visible := p.visibleFormFields()
		currentField := pFieldName
		if p.formFocus >= 0 && p.formFocus < len(visible) {
			currentField = visible[p.formFocus]
		}

		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			p.mode = pickerModeList
			p.err = ""
			return p, nil, PickerNone

		case msg.String() == "enter":
			if currentField == pFieldVersion {
				p.cycleVersion(true)
				return p, nil, PickerNone
			}
			if currentField == pFieldAuthProt {
				p.cycleAuth(true)
				return p, nil, PickerNone
			}
			if currentField == pFieldPrivProt {
				p.cyclePriv(true)
				return p, nil, PickerNone
			}
			// If not on the last visible field, advance focus
			if p.formFocus < len(visible)-1 {
				p.focusFormField(p.formFocus + 1)
				return p, nil, PickerNone
			}
			// On the last field, save
			return p.saveForm()

		case msg.String() == " ":
			if currentField == pFieldVersion {
				p.cycleVersion(true)
				return p, nil, PickerNone
			}
			if currentField == pFieldAuthProt {
				p.cycleAuth(true)
				return p, nil, PickerNone
			}
			if currentField == pFieldPrivProt {
				p.cyclePriv(true)
				return p, nil, PickerNone
			}
			return p.updateActiveFormInput(msg)

		case msg.String() == "tab":
			p.focusFormField(p.formFocus + 1)
			return p, nil, PickerNone

		case msg.String() == "shift+tab":
			p.focusFormField(p.formFocus - 1)
			return p, nil, PickerNone

		default:
			if currentField == pFieldVersion || currentField == pFieldAuthProt || currentField == pFieldPrivProt {
				return p, nil, PickerNone
			}
			return p.updateActiveFormInput(msg)
		}
	}
	return p, nil, PickerNone
}

func (p IdentityPickerModel) updateActiveFormInput(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction) {
	visible := p.visibleFormFields()
	if p.formFocus < 0 || p.formFocus >= len(visible) {
		return p, nil, PickerNone
	}
	fieldIdx := visible[p.formFocus]
	var cmd tea.Cmd
	p.formFields[fieldIdx], cmd = p.formFields[fieldIdx].Update(msg)
	return p, cmd, PickerNone
}

func (p IdentityPickerModel) saveForm() (IdentityPickerModel, tea.Cmd, PickerAction) {
	name := strings.TrimSpace(p.formFields[pFieldName].Value())
	if name == "" {
		p.err = "Name is required"
		return p, nil, PickerNone
	}

	id := identity.Identity{
		Name:    name,
		Version: p.formVersion,
	}

	if p.formVersion == "1" || p.formVersion == "2c" {
		id.Community = p.formFields[pFieldComm].Value()
	} else {
		id.Username = p.formFields[pFieldUser].Value()
		if p.formAuth != "None" {
			id.AuthProto = p.formAuth
			id.AuthPass = p.formFields[pFieldAuthPass].Value()
		}
		if p.formPriv != "None" {
			id.PrivProto = p.formPriv
			id.PrivPass = p.formFields[pFieldPrivPass].Value()
		}
	}

	if p.provider == nil {
		p.err = "No identity store available"
		return p, nil, PickerNone
	}

	if err := p.provider.Add(id); err != nil {
		p.err = fmt.Sprintf("Save failed: %v", err)
		return p, nil, PickerNone
	}

	// Auto-select the newly created identity and return
	p.selectedName = name
	p.refreshList()
	return p, nil, PickerSelected
}

func (p IdentityPickerModel) formFieldLabel(idx int) string {
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

func (p IdentityPickerModel) viewForm() string {
	modalWidth := 50
	if p.width > 70 {
		modalWidth = 56
	}
	if modalWidth < 40 {
		modalWidth = 40
	}
	innerWidth := modalWidth - 6

	var b strings.Builder

	if p.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(p.theme.Base08)
		b.WriteString(errStyle.Render(p.err) + "\n\n")
	}

	visible := p.visibleFormFields()

	labelStyle := p.sty.FormLabel
	activeLabel := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
	cycleHint := lipgloss.NewStyle().Foreground(p.theme.Base04)

	for vi, fieldIdx := range visible {
		isFocused := vi == p.formFocus
		label := p.formFieldLabel(fieldIdx)

		lbl := labelStyle
		if isFocused {
			lbl = activeLabel
		}

		labelText := lbl.Render(padPickerLabel(label+":", 18))

		switch fieldIdx {
		case pFieldVersion, pFieldAuthProt, pFieldPrivProt:
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			valStyle := lipgloss.NewStyle().Foreground(p.theme.Base0C)
			if isFocused {
				valStyle = valStyle.Background(p.theme.Base02)
			}
			var val string
			switch fieldIdx {
			case pFieldVersion:
				val = p.formVersion
			case pFieldAuthProt:
				val = p.formAuth
			case pFieldPrivProt:
				val = p.formPriv
			}
			hint := ""
			if isFocused {
				hint = cycleHint.Render("  (enter/space)")
			}
			b.WriteString(fmt.Sprintf("%s%s%s%s\n", indicator, labelText, valStyle.Render(val), hint))

		default:
			indicator := "  "
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
			}
			b.WriteString(fmt.Sprintf("%s%s%s\n", indicator, labelText, p.formFields[fieldIdx].View()))
		}
	}

	// Help line
	helpStyle := lipgloss.NewStyle().Foreground(p.theme.Base04)
	helpKeyStyle := lipgloss.NewStyle().Foreground(p.theme.Base0D).Bold(true)
	help := fmt.Sprintf(
		"%s/%s navigate  %s save  %s back",
		helpKeyStyle.Render("tab"),
		helpKeyStyle.Render("shift+tab"),
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("esc"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		b.String(),
		"",
		helpStyle.Render(help),
	)

	noTopBorder := p.sty.ModalBorder.BorderTop(false)
	modalBody := noTopBorder.Width(innerWidth).Render(content)

	borderFg := lipgloss.NewStyle().Foreground(p.theme.Base0D).Background(p.theme.Base00)
	titleText := " New Identity "
	titleRendered := p.sty.ModalTitle.Render(titleText)

	fullWidth := lipgloss.Width(modalBody)
	rightDashes := fullWidth - 2 - 1 - len(titleText)
	if rightDashes < 0 {
		rightDashes = 0
	}
	topBorder := borderFg.Render("╭─") + titleRendered + borderFg.Render(strings.Repeat("─", rightDashes)+"╮")

	modal := topBorder + "\n" + modalBody
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, modal)
}

// padPickerLabel right-pads a string to the given width.
func padPickerLabel(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
```

**Step 2: Build and verify compilation**

Run: `make install`
Expected: Clean compilation with no errors.

**Step 3: Commit**

```
git add tui/components/identitypicker.go
git commit -m "Add identity creation form to IdentityPickerModel"
```

---

### Task 3: Integrate picker into BuilderView

**Files:**
- Modify: `tui/views/builder.go`

**Step 1: Add picker fields to BuilderView struct**

Add these fields to the `BuilderView` struct (after line 72):

```go
// Identity picker overlay
showPicker    bool
picker        components.IdentityPickerModel
pickerTarget  string // "step1" or "step2" to know which field to write back to
```

Add import for `"github.com/tonhe/flo/tui/components"` to the imports block.

**Step 2: Modify step 1 update to trigger picker on identity field**

In `updateStepName()` (around line 247, the `"enter"` case):
- When `b.step1Focus == 1` (identity field), set `b.showPicker = true`, `b.pickerTarget = "step1"`, initialize the picker, and return early instead of advancing focus.

Also in `updateStepName()`, add a check at the top: if `b.showPicker`, route all input to the picker.

Replace the `updateStepName` method:

```go
func (b BuilderView) updateStepName(msg tea.Msg) (BuilderView, tea.Cmd, BuilderAction) {
	// Route to picker when active
	if b.showPicker {
		var cmd tea.Cmd
		var action components.PickerAction
		b.picker, cmd, action = b.picker.Update(msg)
		switch action {
		case components.PickerSelected:
			b.identityInput.SetValue(b.picker.SelectedName())
			b.showPicker = false
			// Re-load identities in case a new one was created
			if b.provider != nil {
				b.identities = nil
				if sums, err := b.provider.List(); err == nil {
					for _, s := range sums {
						b.identities = append(b.identities, s.Name)
					}
				}
			}
		case components.PickerCancelled:
			b.showPicker = false
		}
		return b, cmd, BuilderActionNone
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return b, nil, BuilderActionClose

		case msg.String() == "tab":
			b.focusStep1(b.step1Focus + 1)
			return b, nil, BuilderActionNone

		case msg.String() == "shift+tab":
			b.focusStep1(b.step1Focus - 1)
			return b, nil, BuilderActionNone

		case msg.String() == "enter":
			// Identity field opens picker
			if b.step1Focus == 1 {
				b.picker = components.NewIdentityPicker(b.theme, b.provider)
				b.picker.SetSize(b.width, b.height)
				b.showPicker = true
				b.pickerTarget = "step1"
				return b, nil, BuilderActionNone
			}
			// If not on last field, advance focus
			if b.step1Focus < step1Fields-1 {
				b.focusStep1(b.step1Focus + 1)
				return b, nil, BuilderActionNone
			}
			// On last field, validate and advance to step 2
			name := strings.TrimSpace(b.nameInput.Value())
			if name == "" {
				b.err = "Dashboard name is required"
				return b, nil, BuilderActionNone
			}
			b.err = ""
			b.step = StepTargets
			b.addingTarget = true
			b.step2Focus = 0
			b.hostInput.Focus()
			b.labelInput.Blur()
			b.targetIdentity.Blur()
			b.interfacesInput.Blur()
			return b, nil, BuilderActionNone

		default:
			var cmd tea.Cmd
			switch b.step1Focus {
			case 0:
				b.nameInput, cmd = b.nameInput.Update(msg)
			case 1:
				b.identityInput, cmd = b.identityInput.Update(msg)
			case 2:
				b.intervalInput, cmd = b.intervalInput.Update(msg)
			}
			return b, cmd, BuilderActionNone
		}
	}
	return b, nil, BuilderActionNone
}
```

**Step 3: Modify step 2 update to trigger picker on target identity field**

In `updateStepTargets()`, when `b.addingTarget` and user presses enter on field index 2 (target identity), open the picker instead of advancing.

Also add picker routing at the top of `updateStepTargets()`:

```go
// At the top of updateStepTargets, before the switch:
if b.showPicker {
    var cmd tea.Cmd
    var action components.PickerAction
    b.picker, cmd, action = b.picker.Update(msg)
    switch action {
    case components.PickerSelected:
        b.targetIdentity.SetValue(b.picker.SelectedName())
        b.showPicker = false
        if b.provider != nil {
            b.identities = nil
            if sums, err := b.provider.List(); err == nil {
                for _, s := range sums {
                    b.identities = append(b.identities, s.Name)
                }
            }
        }
    case components.PickerCancelled:
        b.showPicker = false
    }
    return b, cmd, BuilderActionNone
}
```

In the `"enter"` case when `b.addingTarget` and `b.step2Focus == 2`:

```go
case msg.String() == "enter":
    if b.addingTarget {
        // Identity field opens picker
        if b.step2Focus == 2 {
            b.picker = components.NewIdentityPicker(b.theme, b.provider)
            b.picker.SetSize(b.width, b.height)
            b.showPicker = true
            b.pickerTarget = "step2"
            return b, nil, BuilderActionNone
        }
        // ... rest of existing logic
    }
```

**Step 4: Modify View methods to overlay picker**

In `viewStepName()`, after building the normal view, if `b.showPicker`, return the picker's `View()` instead:

```go
func (b BuilderView) viewStepName() string {
    if b.showPicker {
        return b.picker.View()
    }
    // ... existing code
}
```

Same for `viewStepTargets()`:

```go
func (b BuilderView) viewStepTargets() string {
    if b.showPicker {
        return b.picker.View()
    }
    // ... existing code
}
```

**Step 5: Remove the "Available identities" hint line**

In `viewStepName()`, remove lines 334-339 (the hint block):

```go
// Remove this block:
if len(b.identities) > 0 {
    hintStyle := lipgloss.NewStyle().Foreground(b.theme.Base04)
    s.WriteString("\n")
    s.WriteString("  " + hintStyle.Render("Available identities: "+strings.Join(b.identities, ", ")) + "\n")
}
```

**Step 6: Build and verify**

Run: `make install`
Expected: Clean compilation.

**Step 7: Commit**

```
git add tui/views/builder.go
git commit -m "Integrate identity picker into builder view"
```

---

### Task 4: Integrate picker into EditorView

**Files:**
- Modify: `tui/views/editor.go`

**Step 1: Add picker fields to EditorView struct**

Add these fields to `EditorView` (after line 96):

```go
// Identity picker overlay
showPicker   bool
picker       components.IdentityPickerModel
pickerTarget string // "settings" or "host"
```

Add `"github.com/tonhe/flo/tui/components"` to the imports.

**Step 2: Modify startInlineEdit to trigger picker for identity**

In `startInlineEdit()` (line 230), when `e.settingsCur == edSettingsIdentity`, open the picker instead:

```go
func (e *EditorView) startInlineEdit() {
    if e.settingsCur == edSettingsIdentity {
        e.picker = components.NewIdentityPicker(e.theme, e.provider)
        e.picker.SetSize(e.width, e.height)
        e.showPicker = true
        e.pickerTarget = "settings"
        return
    }
    e.mode = modeInlineEdit
    // ... rest of existing code for non-identity fields
    e.input = textinput.New()
    e.input.CharLimit = 128
    e.input.Width = 40
    e.input.Focus()
    switch e.settingsCur {
    case edSettingsName:
        e.input.SetValue(e.dashName)
        e.editingWhat = "Dashboard Name"
    case edSettingsInterval:
        e.input.SetValue(e.intervalStr)
        e.editingWhat = "Poll Interval"
    }
}
```

**Step 3: Modify startHostInlineEdit to trigger picker for host identity**

In `startHostInlineEdit()` (line 336), when `e.detailCur == edHostFieldIdentity`, open the picker instead:

```go
func (e *EditorView) startHostInlineEdit() {
    if e.detailCur == edHostFieldIdentity {
        e.picker = components.NewIdentityPicker(e.theme, e.provider)
        e.picker.SetSize(e.width, e.height)
        e.showPicker = true
        e.pickerTarget = "host"
        return
    }
    e.mode = modeHostInline
    // ... rest of existing code for non-identity fields
}
```

**Step 4: Modify updateAddHost to trigger picker for identity step**

In `updateAddHost()` (line 419), when `e.detailCur == 2` (identity step) and user presses enter, open the picker instead:

```go
case 2:
    // Open identity picker instead of saving raw text
    e.picker = components.NewIdentityPicker(e.theme, e.provider)
    e.picker.SetSize(e.width, e.height)
    e.showPicker = true
    e.pickerTarget = "addhost"
    return e, nil, EditorActionNone
```

Wait: actually we need to keep it simpler. The enter key in addHost mode case 2 should open the picker. Then the picker result writes back to the target identity and advances to step 3.

**Step 5: Add picker routing to Update method**

In the `Update()` method (line 140), add a picker check at the top before the mode switch:

```go
func (e EditorView) Update(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
    if e.showPicker {
        var cmd tea.Cmd
        var action components.PickerAction
        e.picker, cmd, action = e.picker.Update(msg)
        switch action {
        case components.PickerSelected:
            name := e.picker.SelectedName()
            switch e.pickerTarget {
            case "settings":
                e.defaultIdentity = name
            case "host":
                e.targets[e.detailHostIdx].Identity = name
            case "addhost":
                e.targets[len(e.targets)-1].Identity = name
                e.detailCur = 3
                e.input.SetValue("")
                e.input.Placeholder = "Gi0/0, Gi0/1, Eth1"
                e.editingWhat = "Interfaces (comma-separated)"
            }
            e.showPicker = false
            // Refresh identities list
            if e.provider != nil {
                e.identities = nil
                if sums, listErr := e.provider.List(); listErr == nil {
                    for _, s := range sums {
                        e.identities = append(e.identities, s.Name)
                    }
                }
            }
        case components.PickerCancelled:
            e.showPicker = false
        }
        return e, cmd, EditorActionNone
    }

    switch e.mode {
    // ... existing cases
    }
}
```

**Step 6: Overlay picker in View method**

In `View()` (line 539), add picker overlay:

```go
func (e EditorView) View() string {
    if e.showPicker {
        return e.picker.View()
    }
    switch e.mode {
    // ... existing cases
    }
}
```

**Step 7: Build and verify**

Run: `make install`
Expected: Clean compilation.

**Step 8: Commit**

```
git add tui/views/editor.go
git commit -m "Integrate identity picker into editor view"
```

---

### Task 5: Integrate picker into SettingsView

**Files:**
- Modify: `tui/views/settings.go`

**Step 1: Add picker fields to SettingsView struct**

Add these fields to `SettingsView` (after line 60):

```go
// Identity picker overlay
showPicker bool
picker     components.IdentityPickerModel
provider   identity.Provider
```

Add `"github.com/tonhe/flo/tui/components"` and `"github.com/tonhe/flo/internal/identity"` to imports.

**Step 2: Modify NewSettingsView to accept provider**

Change the function signature and store the provider:

```go
func NewSettingsView(theme styles.Theme, cfg *config.Config, provider identity.Provider) SettingsView {
    // ... existing code
    return SettingsView{
        // ... existing fields
        provider: provider,
    }
}
```

**Step 3: Update callers of NewSettingsView**

In `tui/app.go` line 227, update the call:

```go
m.settings = views.NewSettingsView(m.theme, m.config, m.provider)
```

Also in `tui/app.go` line 367, update the theme-change rebuild:

```go
// The settings view is not rebuilt here (it's being replaced by StateDashboard)
// No change needed.
```

**Step 4: Modify Update to route to picker and trigger picker on identity field**

In `Update()` (line 143), add picker routing at the top and intercept enter on identity field:

```go
func (s SettingsView) Update(msg tea.Msg) (SettingsView, tea.Cmd, SettingsAction) {
    if s.showPicker {
        var cmd tea.Cmd
        var action components.PickerAction
        s.picker, cmd, action = s.picker.Update(msg)
        switch action {
        case components.PickerSelected:
            s.identityInput.SetValue(s.picker.SelectedName())
            s.showPicker = false
        case components.PickerCancelled:
            s.showPicker = false
        }
        return s, cmd, SettingsNone
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch {
        // ... existing cases, but modify the Enter case:
        case key.Matches(msg, keys.DefaultKeyMap.Enter):
            // Identity field opens picker
            if s.cursor == settingsFieldIdentity {
                s.picker = components.NewIdentityPicker(s.theme, s.provider)
                s.picker.SetSize(s.width, s.height)
                s.showPicker = true
                return s, nil, SettingsNone
            }
            return s.save()
        // ... rest of existing cases
        }
    }
}
```

**Step 5: Overlay picker in View**

In `View()` (line 293), add picker overlay:

```go
func (s SettingsView) View() string {
    if s.showPicker {
        return s.picker.View()
    }
    // ... existing code
}
```

**Step 6: Build and verify**

Run: `make install`
Expected: Clean compilation.

**Step 7: Commit**

```
git add tui/views/settings.go tui/app.go
git commit -m "Integrate identity picker into settings view"
```

---

### Task 6: Manual testing and polish

**Files:**
- Potentially modify: `tui/components/identitypicker.go`, any view file

**Step 1: Run the app and test all picker integration points**

Run: `~/bin/flo`

Test each path:
1. Press `s` for settings, navigate to Default Identity, press enter -> picker should appear
2. Press `d` for dashboards, `n` for new dashboard, tab to Identity field, press enter -> picker should appear
3. Continue to Step 2, add a target, tab to Identity field, press enter -> picker should appear
4. Press `e` to edit a dashboard, navigate to a host, enter to open detail, navigate to Identity, enter -> picker should appear
5. In any picker, select "+ New Identity", fill the form, verify it saves and auto-selects
6. In any picker, select "(none)", verify the field is cleared
7. In any picker, press esc, verify it cancels cleanly

**Step 2: Fix any visual or behavioral issues found during testing**

**Step 3: Build final binary**

Run: `make install`
Expected: Clean compilation.

**Step 4: Commit any fixes**

```
git add -A
git commit -m "Polish identity picker visual and behavioral fixes"
```

---

### Task 7: Update ROADMAP.md and CHANGELOG.md

**Files:**
- Modify: `ROADMAP.md`
- Modify: `CHANGELOG.md`

**Step 1: Mark the roadmap item as complete**

Change `- [ ] Add identity picker interface for confiruing defaults, or host identities.` to `- [x] Add identity picker interface for configuring defaults, or host identities.`

**Step 2: Add changelog entry**

Add to the top of `CHANGELOG.md`:

```markdown
## [Unreleased]

### Added
- Identity picker modal overlay replaces free-text identity inputs in builder, editor, and settings views
- Inline identity creation form within the picker — create new identities without leaving your current view
- "(none)" option in picker to clear identity assignments and use fallback behavior
```

**Step 3: Commit**

```
git add ROADMAP.md CHANGELOG.md
git commit -m "Update roadmap and changelog for identity picker"
```
