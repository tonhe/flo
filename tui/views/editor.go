package views

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// EditorAction describes what the app should do after an editor update.
type EditorAction int

const (
	// EditorActionNone means continue in the editor.
	EditorActionNone EditorAction = iota
	// EditorActionClose means the user cancelled without saving.
	EditorActionClose
	// EditorActionSaved means the dashboard was saved; SavedPath has the file.
	EditorActionSaved
)

type editorSection int

const (
	sectionSettings editorSection = iota
	sectionHosts
)

type editorMode int

const (
	modeMenu       editorMode = iota
	modeInlineEdit
	modeHostDetail
	modeHostInline
	modeAddHost
)

// Settings field indices for the editor menu.
const (
	edSettingsName     = 0
	edSettingsIdentity = 1
	edSettingsInterval = 2
	edSettingsCount    = 3
)

// Host detail field indices.
const (
	edHostFieldHost       = 0
	edHostFieldLabel      = 1
	edHostFieldIdentity   = 2
	edHostFieldInterfaces = 3
)

// EditorView is an nbor-style inline editor for existing dashboards.
type EditorView struct {
	theme    styles.Theme
	sty      *styles.Styles
	provider identity.Provider
	width    int
	height   int

	dashName        string
	defaultIdentity string
	intervalStr     string
	targets         []dashboard.Target
	editPath        string

	mode        editorMode
	section     editorSection
	settingsCur int
	hostsCur    int
	globalCur   int

	detailHostIdx int
	detailCur     int

	input       textinput.Model
	editingWhat string

	identities []string

	addHostBaseLen int // target count before add-host started

	SavedPath string
	err       string
}

// NewEditorView creates a new EditorView with the given theme and identity provider.
func NewEditorView(theme styles.Theme, provider identity.Provider) EditorView {
	sty := styles.NewStyles(theme)
	e := EditorView{
		theme:    theme,
		sty:      sty,
		provider: provider,
		mode:     modeMenu,
		section:  sectionSettings,
	}
	if provider != nil {
		if sums, listErr := provider.List(); listErr == nil {
			for _, s := range sums {
				e.identities = append(e.identities, s.Name)
			}
		}
	}
	return e
}

// LoadDashboard populates the editor from an existing dashboard.
func (e *EditorView) LoadDashboard(dash *dashboard.Dashboard, path string) {
	e.editPath = path
	e.dashName = dash.Name
	e.defaultIdentity = dash.DefaultIdentity
	e.intervalStr = dash.Interval.String()
	e.targets = nil
	for _, g := range dash.Groups {
		for _, t := range g.Targets {
			e.targets = append(e.targets, t)
		}
	}
}

// SetSize updates the available dimensions for the editor view.
func (e *EditorView) SetSize(width, height int) {
	e.width = width
	e.height = height
}

// Update handles messages for the editor and dispatches by mode.
func (e EditorView) Update(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	switch e.mode {
	case modeMenu:
		return e.updateMenu(msg)
	case modeInlineEdit:
		return e.updateInlineEdit(msg)
	case modeHostDetail:
		return e.updateHostDetail(msg)
	case modeHostInline:
		return e.updateHostInline(msg)
	case modeAddHost:
		return e.updateAddHost(msg)
	}
	return e, nil, EditorActionNone
}

// --- Menu mode ---

func (e EditorView) updateMenu(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		totalRows := edSettingsCount + len(e.targets)

		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			if err := e.save(); err != nil {
				e.err = err.Error()
				return e, nil, EditorActionNone
			}
			return e, nil, EditorActionSaved

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if e.globalCur > 0 {
				e.globalCur--
			}
			e.syncCursorFromGlobal()
			return e, nil, EditorActionNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if e.globalCur < totalRows-1 {
				e.globalCur++
			}
			e.syncCursorFromGlobal()
			return e, nil, EditorActionNone

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			if e.section == sectionSettings {
				e.startInlineEdit()
				return e, nil, EditorActionNone
			}
			if e.section == sectionHosts && len(e.targets) > 0 {
				e.detailHostIdx = e.hostsCur
				e.detailCur = 0
				e.mode = modeHostDetail
				return e, nil, EditorActionNone
			}
			return e, nil, EditorActionNone

		case msg.String() == "a":
			e.mode = modeAddHost
			e.initAddHostInputs()
			return e, nil, EditorActionNone

		case msg.String() == "d":
			if e.section == sectionHosts && len(e.targets) > 0 {
				e.targets = append(e.targets[:e.hostsCur], e.targets[e.hostsCur+1:]...)
				if e.hostsCur >= len(e.targets) && e.hostsCur > 0 {
					e.hostsCur--
					e.globalCur--
				}
				e.syncCursorFromGlobal()
			}
			return e, nil, EditorActionNone
		}
	}
	return e, nil, EditorActionNone
}

func (e *EditorView) syncCursorFromGlobal() {
	if e.globalCur < edSettingsCount {
		e.section = sectionSettings
		e.settingsCur = e.globalCur
	} else {
		e.section = sectionHosts
		e.hostsCur = e.globalCur - edSettingsCount
	}
}

// --- Inline edit for settings fields ---

func (e *EditorView) startInlineEdit() {
	e.mode = modeInlineEdit
	e.input = textinput.New()
	e.input.CharLimit = 128
	e.input.Width = 40
	e.input.Focus()
	switch e.settingsCur {
	case edSettingsName:
		e.input.SetValue(e.dashName)
		e.editingWhat = "Dashboard Name"
	case edSettingsIdentity:
		e.input.SetValue(e.defaultIdentity)
		e.editingWhat = "Default Identity"
	case edSettingsInterval:
		e.input.SetValue(e.intervalStr)
		e.editingWhat = "Poll Interval"
	}
}

func (e EditorView) updateInlineEdit(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			e.mode = modeMenu
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			val := strings.TrimSpace(e.input.Value())
			switch e.settingsCur {
			case edSettingsName:
				if val == "" {
					e.err = "Dashboard name is required"
					return e, nil, EditorActionNone
				}
				e.dashName = val
			case edSettingsIdentity:
				e.defaultIdentity = val
			case edSettingsInterval:
				if _, parseErr := time.ParseDuration(val); parseErr != nil {
					e.err = fmt.Sprintf("Invalid interval: %v", parseErr)
					return e, nil, EditorActionNone
				}
				e.intervalStr = val
			}
			e.err = ""
			e.mode = modeMenu
			return e, nil, EditorActionNone
		default:
			var cmd tea.Cmd
			e.input, cmd = e.input.Update(msg)
			return e, cmd, EditorActionNone
		}
	}
	return e, nil, EditorActionNone
}

// --- Host detail sub-view ---

func (e EditorView) updateHostDetail(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	target := e.targets[e.detailHostIdx]
	totalRows := 3 + len(target.Interfaces)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			e.mode = modeMenu
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if e.detailCur > 0 {
				e.detailCur--
			}
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if e.detailCur < totalRows-1 {
				e.detailCur++
			}
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			e.startHostInlineEdit()
			return e, nil, EditorActionNone
		case msg.String() == "a":
			if e.detailCur >= edHostFieldInterfaces || len(target.Interfaces) == 0 {
				e.targets[e.detailHostIdx].Interfaces = append(e.targets[e.detailHostIdx].Interfaces, "")
				idx := len(e.targets[e.detailHostIdx].Interfaces) - 1
				e.detailCur = edHostFieldInterfaces + idx
				e.startHostInlineEdit()
			}
			return e, nil, EditorActionNone
		case msg.String() == "d":
			if e.detailCur >= edHostFieldInterfaces {
				ifIdx := e.detailCur - edHostFieldInterfaces
				ifaces := e.targets[e.detailHostIdx].Interfaces
				if ifIdx < len(ifaces) {
					e.targets[e.detailHostIdx].Interfaces = append(ifaces[:ifIdx], ifaces[ifIdx+1:]...)
					if e.detailCur >= edHostFieldInterfaces+len(e.targets[e.detailHostIdx].Interfaces) && e.detailCur > 0 {
						e.detailCur--
					}
				}
			}
			return e, nil, EditorActionNone
		}
	}
	return e, nil, EditorActionNone
}

func (e *EditorView) startHostInlineEdit() {
	e.mode = modeHostInline
	e.input = textinput.New()
	e.input.CharLimit = 256
	e.input.Width = 40
	e.input.Focus()
	target := e.targets[e.detailHostIdx]
	switch {
	case e.detailCur == edHostFieldHost:
		e.input.SetValue(target.Host)
		e.editingWhat = "Host"
	case e.detailCur == edHostFieldLabel:
		e.input.SetValue(target.Label)
		e.editingWhat = "Label"
	case e.detailCur == edHostFieldIdentity:
		e.input.SetValue(target.Identity)
		e.editingWhat = "Identity"
	default:
		ifIdx := e.detailCur - edHostFieldInterfaces
		if ifIdx < len(target.Interfaces) {
			e.input.SetValue(target.Interfaces[ifIdx])
		}
		e.editingWhat = "Interface"
	}
}

func (e EditorView) updateHostInline(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			e.mode = modeHostDetail
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			val := strings.TrimSpace(e.input.Value())
			switch {
			case e.detailCur == edHostFieldHost:
				if val == "" {
					e.err = "Host is required"
					return e, nil, EditorActionNone
				}
				e.targets[e.detailHostIdx].Host = val
			case e.detailCur == edHostFieldLabel:
				e.targets[e.detailHostIdx].Label = val
			case e.detailCur == edHostFieldIdentity:
				e.targets[e.detailHostIdx].Identity = val
			default:
				ifIdx := e.detailCur - edHostFieldInterfaces
				if val == "" {
					ifaces := e.targets[e.detailHostIdx].Interfaces
					e.targets[e.detailHostIdx].Interfaces = append(ifaces[:ifIdx], ifaces[ifIdx+1:]...)
					if e.detailCur > 0 {
						e.detailCur--
					}
				} else {
					e.targets[e.detailHostIdx].Interfaces[ifIdx] = val
				}
			}
			e.err = ""
			e.mode = modeHostDetail
			return e, nil, EditorActionNone
		default:
			var cmd tea.Cmd
			e.input, cmd = e.input.Update(msg)
			return e, cmd, EditorActionNone
		}
	}
	return e, nil, EditorActionNone
}

// --- Add host mode ---

func (e *EditorView) initAddHostInputs() {
	e.addHostBaseLen = len(e.targets)
	e.input = textinput.New()
	e.input.Placeholder = "host IP or hostname"
	e.input.CharLimit = 128
	e.input.Width = 40
	e.input.Focus()
	e.editingWhat = "Host"
	e.detailCur = 0
}

func (e EditorView) updateAddHost(msg tea.Msg) (EditorView, tea.Cmd, EditorAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			// Remove any partial target added during this add-host session
			if len(e.targets) > e.addHostBaseLen {
				e.targets = e.targets[:e.addHostBaseLen]
			}
			e.mode = modeMenu
			return e, nil, EditorActionNone
		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			val := strings.TrimSpace(e.input.Value())
			switch e.detailCur {
			case 0:
				if val == "" {
					e.err = "Host is required"
					return e, nil, EditorActionNone
				}
				e.targets = append(e.targets, dashboard.Target{Host: val, Label: val, Port: 161})
				e.detailCur = 1
				e.input.SetValue(val)
				e.input.Placeholder = "label (display name)"
				e.editingWhat = "Label"
			case 1:
				if val != "" {
					e.targets[len(e.targets)-1].Label = val
				}
				e.detailCur = 2
				e.input.SetValue("")
				e.input.Placeholder = "identity override (optional)"
				e.editingWhat = "Identity"
			case 2:
				e.targets[len(e.targets)-1].Identity = val
				e.detailCur = 3
				e.input.SetValue("")
				e.input.Placeholder = "Gi0/0, Gi0/1, Eth1"
				e.editingWhat = "Interfaces (comma-separated)"
			case 3:
				if val == "" {
					e.err = "At least one interface is required"
					return e, nil, EditorActionNone
				}
				var ifaces []string
				for _, part := range strings.Split(val, ",") {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						ifaces = append(ifaces, trimmed)
					}
				}
				e.targets[len(e.targets)-1].Interfaces = ifaces
				e.err = ""
				e.globalCur = edSettingsCount + len(e.targets) - 1
				e.syncCursorFromGlobal()
				e.mode = modeMenu
			}
			return e, nil, EditorActionNone
		default:
			var cmd tea.Cmd
			e.input, cmd = e.input.Update(msg)
			return e, cmd, EditorActionNone
		}
	}
	return e, nil, EditorActionNone
}

// --- Save ---

func (e *EditorView) save() error {
	if e.dashName == "" {
		return fmt.Errorf("dashboard name is required")
	}
	intervalStr := e.intervalStr
	if intervalStr == "" {
		intervalStr = "10s"
	}
	interval, parseErr := time.ParseDuration(intervalStr)
	if parseErr != nil {
		return fmt.Errorf("invalid interval: %v", parseErr)
	}
	targets := make([]dashboard.Target, len(e.targets))
	copy(targets, e.targets)
	for i := range targets {
		if targets[i].Identity == "" {
			targets[i].Identity = e.defaultIdentity
		}
		if targets[i].Port == 0 {
			targets[i].Port = 161
		}
	}
	dash := &dashboard.Dashboard{
		Name:            e.dashName,
		DefaultIdentity: e.defaultIdentity,
		Interval:        interval,
		MaxHistory:      360,
		Groups: []dashboard.Group{
			{Name: "Default", Targets: targets},
		},
	}
	path := e.editPath
	if path == "" {
		dashDir, dirErr := config.GetDashboardsDir()
		if dirErr != nil {
			return fmt.Errorf("failed to get dashboards dir: %v", dirErr)
		}
		if mkdirErr := config.EnsureDirs(); mkdirErr != nil {
			return fmt.Errorf("failed to create directories: %v", mkdirErr)
		}
		path = filepath.Join(dashDir, slugify(e.dashName)+".toml")
	}
	if saveErr := dashboard.SaveDashboard(dash, path); saveErr != nil {
		return fmt.Errorf("failed to save: %v", saveErr)
	}
	e.SavedPath = path
	return nil
}

// --- View methods ---

// View renders the editor based on the current mode.
func (e EditorView) View() string {
	switch e.mode {
	case modeMenu, modeInlineEdit:
		return e.viewMenu()
	case modeHostDetail, modeHostInline:
		return e.viewHostDetail()
	case modeAddHost:
		return e.viewAddHost()
	}
	return ""
}

func (e EditorView) viewMenu() string {
	titleStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	labelStyle := e.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(e.theme.Base06)
	cursorStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(e.theme.Base03)

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render(fmt.Sprintf("Edit Dashboard: %s", e.dashName)) + "\n\n")

	if e.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(e.theme.Base08)
		s.WriteString("  " + errStyle.Render(e.err) + "\n\n")
	}

	s.WriteString("  " + sectionStyle.Render("--- Settings ---") + "\n\n")

	settingsRows := []struct{ label, value string }{
		{"Dashboard Name", e.dashName},
		{"Default Identity", e.defaultIdentity},
		{"Poll Interval", e.intervalStr},
	}

	for i, row := range settingsRows {
		isActive := e.section == sectionSettings && e.settingsCur == i
		indicator := "  "
		lbl := labelStyle
		if isActive {
			indicator = cursorStyle.Render("> ")
			lbl = activeLabelStyle
		}
		val := valStyle.Render(row.value)
		if row.value == "" {
			val = dimStyle.Render("(none)")
		}
		if isActive && e.mode == modeInlineEdit {
			val = e.input.View()
		}
		s.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(row.label, 22)), val))
	}

	s.WriteString("\n  " + sectionStyle.Render("--- Hosts ---") + "\n\n")

	if len(e.targets) == 0 {
		s.WriteString("  " + dimStyle.Render("  No hosts configured. Press [a] to add.") + "\n")
	} else {
		for i, t := range e.targets {
			isActive := e.section == sectionHosts && e.hostsCur == i
			indicator := "  "
			lbl := labelStyle
			if isActive {
				indicator = cursorStyle.Render("> ")
				lbl = activeLabelStyle
			}
			ifaceStr := strings.Join(t.Interfaces, ", ")
			if len(ifaceStr) > 30 {
				ifaceStr = ifaceStr[:27] + "..."
			}
			label := t.Label
			if label == "" {
				label = t.Host
			}
			s.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(label, 22)), valStyle.Render(ifaceStr)))
		}
	}

	s.WriteString("\n  " + e.renderMenuHelp() + "\n")
	return s.String()
}

func (e EditorView) renderMenuHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(e.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	if e.mode == modeInlineEdit {
		return helpStyle.Render(fmt.Sprintf("%s commit  %s cancel",
			keyStyle.Render("[enter]"), keyStyle.Render("[esc]")))
	}
	return helpStyle.Render(fmt.Sprintf("%s add host  %s edit selected  %s delete host  %s save & close",
		keyStyle.Render("[a]"), keyStyle.Render("[enter]"), keyStyle.Render("[d]"), keyStyle.Render("[esc]")))
}

func (e EditorView) viewHostDetail() string {
	target := e.targets[e.detailHostIdx]
	titleStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	labelStyle := e.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(e.theme.Base06)
	cursorStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(e.theme.Base03)

	label := target.Label
	if label == "" {
		label = target.Host
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render(fmt.Sprintf("Edit Host: %s", label)) + "\n\n")

	if e.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(e.theme.Base08)
		s.WriteString("  " + errStyle.Render(e.err) + "\n\n")
	}

	hostFields := []struct{ label, value string }{
		{"Host", target.Host},
		{"Label", target.Label},
		{"Identity", target.Identity},
	}

	for i, f := range hostFields {
		isActive := e.detailCur == i
		indicator := "  "
		lbl := labelStyle
		if isActive {
			indicator = cursorStyle.Render("> ")
			lbl = activeLabelStyle
		}
		val := valStyle.Render(f.value)
		if f.value == "" {
			if i == 2 {
				val = dimStyle.Render("(default)")
			} else {
				val = dimStyle.Render("(none)")
			}
		}
		if isActive && e.mode == modeHostInline {
			val = e.input.View()
		}
		s.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(f.label, 22)), val))
	}

	s.WriteString("\n  " + sectionStyle.Render("--- Interfaces ---") + "\n\n")

	if len(target.Interfaces) == 0 {
		s.WriteString("  " + dimStyle.Render("  No interfaces. Press [a] to add.") + "\n")
	} else {
		for i, ifName := range target.Interfaces {
			rowIdx := edHostFieldInterfaces + i
			isActive := e.detailCur == rowIdx
			indicator := "  "
			lbl := labelStyle
			if isActive {
				indicator = cursorStyle.Render("> ")
				lbl = activeLabelStyle
			}
			val := lbl.Render(ifName)
			if isActive && e.mode == modeHostInline {
				val = e.input.View()
			}
			s.WriteString(fmt.Sprintf("  %s%s\n", indicator, val))
		}
	}

	s.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(e.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	if e.mode == modeHostInline {
		s.WriteString("  " + helpStyle.Render(fmt.Sprintf("%s commit  %s cancel",
			keyStyle.Render("[enter]"), keyStyle.Render("[esc]"))) + "\n")
	} else {
		s.WriteString("  " + helpStyle.Render(fmt.Sprintf("%s add interface  %s edit  %s delete  %s back",
			keyStyle.Render("[a]"), keyStyle.Render("[enter]"), keyStyle.Render("[d]"), keyStyle.Render("[esc]"))) + "\n")
	}
	return s.String()
}

func (e EditorView) viewAddHost() string {
	titleStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	labelStyle := e.sty.FormLabel

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render("Add Host") + "\n\n")

	if e.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(e.theme.Base08)
		s.WriteString("  " + errStyle.Render(e.err) + "\n\n")
	}

	s.WriteString(fmt.Sprintf("  %s%s\n", labelStyle.Render(padRight(e.editingWhat+":", 22)), e.input.View()))

	s.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(e.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(e.theme.Base0D).Bold(true)
	s.WriteString("  " + helpStyle.Render(fmt.Sprintf("%s next  %s cancel",
		keyStyle.Render("[enter]"), keyStyle.Render("[esc]"))) + "\n")
	return s.String()
}
