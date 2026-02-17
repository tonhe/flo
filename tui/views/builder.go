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

// BuilderStep represents which step the wizard is on.
type BuilderStep int

const (
	StepName    BuilderStep = iota // Dashboard name & defaults
	StepTargets                    // Add targets
	StepReview                     // Review & save
)

// BuilderAction describes what the app should do after a builder update.
type BuilderAction int

const (
	// BuilderActionNone means continue in the builder.
	BuilderActionNone BuilderAction = iota
	// BuilderActionClose means the user cancelled.
	BuilderActionClose
	// BuilderActionSave means the dashboard was saved; SavedPath has the file.
	BuilderActionSave
)

// BuilderView is a step-by-step wizard for creating new dashboards.
type BuilderView struct {
	theme    styles.Theme
	sty      *styles.Styles
	provider identity.Provider
	step     BuilderStep
	width    int
	height   int

	// Step 1: Name & defaults
	nameInput     textinput.Model
	intervalInput textinput.Model
	identityInput textinput.Model
	identities    []string // available identity names from provider
	step1Focus    int

	// Step 2: Targets
	hostInput       textinput.Model
	labelInput      textinput.Model
	targetIdentity  textinput.Model
	interfacesInput textinput.Model
	targets         []dashboard.Target
	targetCursor    int
	step2Focus      int
	addingTarget    bool

	// Step 3: Review
	SavedPath string // path to the saved TOML file after save
	err       string
}

// step1 field count
const step1Fields = 3

// step2 field count (host, label, identity override, interfaces)
const step2Fields = 4

// NewBuilderView creates a fresh BuilderView ready for use.
func NewBuilderView(theme styles.Theme, provider identity.Provider) BuilderView {
	sty := styles.NewStyles(theme)

	b := BuilderView{
		theme:    theme,
		sty:      sty,
		provider: provider,
		step:     StepName,
	}

	// Load available identity names
	if provider != nil {
		if sums, err := provider.List(); err == nil {
			for _, s := range sums {
				b.identities = append(b.identities, s.Name)
			}
		}
	}

	// Step 1 inputs
	b.nameInput = textinput.New()
	b.nameInput.Placeholder = "dashboard name"
	b.nameInput.CharLimit = 64
	b.nameInput.Width = 40
	b.nameInput.Focus()

	b.identityInput = textinput.New()
	b.identityInput.Placeholder = "default identity"
	b.identityInput.CharLimit = 64
	b.identityInput.Width = 40

	b.intervalInput = textinput.New()
	b.intervalInput.Placeholder = "10s"
	b.intervalInput.CharLimit = 16
	b.intervalInput.Width = 40
	b.intervalInput.SetValue("10s")

	b.step1Focus = 0

	// Step 2 inputs
	b.hostInput = textinput.New()
	b.hostInput.Placeholder = "host IP or hostname"
	b.hostInput.CharLimit = 128
	b.hostInput.Width = 40

	b.labelInput = textinput.New()
	b.labelInput.Placeholder = "label (display name)"
	b.labelInput.CharLimit = 64
	b.labelInput.Width = 40

	b.targetIdentity = textinput.New()
	b.targetIdentity.Placeholder = "identity override (optional)"
	b.targetIdentity.CharLimit = 64
	b.targetIdentity.Width = 40

	b.interfacesInput = textinput.New()
	b.interfacesInput.Placeholder = "Gi0/0, Gi0/1, Eth1"
	b.interfacesInput.CharLimit = 256
	b.interfacesInput.Width = 40

	b.addingTarget = true
	b.step2Focus = 0

	return b
}

// SetSize updates the available dimensions for the builder view.
func (b *BuilderView) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// Update handles messages for the builder wizard.
func (b BuilderView) Update(msg tea.Msg) (BuilderView, tea.Cmd, BuilderAction) {
	switch b.step {
	case StepName:
		return b.updateStepName(msg)
	case StepTargets:
		return b.updateStepTargets(msg)
	case StepReview:
		return b.updateStepReview(msg)
	}
	return b, nil, BuilderActionNone
}

// View renders the current step of the builder wizard.
func (b BuilderView) View() string {
	switch b.step {
	case StepName:
		return b.viewStepName()
	case StepTargets:
		return b.viewStepTargets()
	case StepReview:
		return b.viewStepReview()
	}
	return ""
}

// --- Step 1: Name & Defaults ---

func (b *BuilderView) focusStep1(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= step1Fields {
		idx = step1Fields - 1
	}
	b.step1Focus = idx

	b.nameInput.Blur()
	b.identityInput.Blur()
	b.intervalInput.Blur()

	switch idx {
	case 0:
		b.nameInput.Focus()
	case 1:
		b.identityInput.Focus()
	case 2:
		b.intervalInput.Focus()
	}
}

func (b BuilderView) updateStepName(msg tea.Msg) (BuilderView, tea.Cmd, BuilderAction) {
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
			// Pass to focused input
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

func (b BuilderView) viewStepName() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base0D).
		Bold(true)
	stepStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base04)
	labelStyle := b.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base0D).
		Bold(true)

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render("New Dashboard") + "  " + stepStyle.Render("Step 1 of 3") + "\n")
	s.WriteString("\n")

	if b.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(b.theme.Base08)
		s.WriteString("  " + errStyle.Render(b.err) + "\n\n")
	}

	type field struct {
		label string
		view  string
	}
	fields := []field{
		{"Name", b.nameInput.View()},
		{"Identity", b.identityInput.View()},
		{"Poll Interval", b.intervalInput.View()},
	}

	for i, f := range fields {
		isFocused := i == b.step1Focus
		indicator := "  "
		lbl := labelStyle
		if isFocused {
			indicatorStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)
			indicator = indicatorStyle.Render("> ")
			lbl = activeLabelStyle
		}
		s.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(f.label+":", 18)), f.view))
	}

	// Show available identities as a hint
	if len(b.identities) > 0 {
		hintStyle := lipgloss.NewStyle().Foreground(b.theme.Base04)
		s.WriteString("\n")
		s.WriteString("  " + hintStyle.Render("Available identities: "+strings.Join(b.identities, ", ")) + "\n")
	}

	s.WriteString("\n")
	s.WriteString("  " + b.renderStep1Help() + "\n")

	return s.String()
}

func (b BuilderView) renderStep1Help() string {
	helpStyle := lipgloss.NewStyle().Foreground(b.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)
	return helpStyle.Render(fmt.Sprintf(
		"%s/%s navigate  %s next  %s cancel",
		keyStyle.Render("[tab]"),
		keyStyle.Render("[shift+tab]"),
		keyStyle.Render("[enter]"),
		keyStyle.Render("[esc]"),
	))
}

// --- Step 2: Add Targets ---

func (b *BuilderView) focusStep2(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx >= step2Fields {
		idx = step2Fields - 1
	}
	b.step2Focus = idx

	b.hostInput.Blur()
	b.labelInput.Blur()
	b.targetIdentity.Blur()
	b.interfacesInput.Blur()

	switch idx {
	case 0:
		b.hostInput.Focus()
	case 1:
		b.labelInput.Focus()
	case 2:
		b.targetIdentity.Focus()
	case 3:
		b.interfacesInput.Focus()
	}
}

func (b *BuilderView) resetTargetForm() {
	b.hostInput.SetValue("")
	b.labelInput.SetValue("")
	b.targetIdentity.SetValue("")
	b.interfacesInput.SetValue("")
	b.step2Focus = 0
	b.hostInput.Focus()
	b.labelInput.Blur()
	b.targetIdentity.Blur()
	b.interfacesInput.Blur()
}

func (b *BuilderView) commitCurrentTarget() bool {
	host := strings.TrimSpace(b.hostInput.Value())
	if host == "" {
		b.err = "Host is required"
		return false
	}

	label := strings.TrimSpace(b.labelInput.Value())
	if label == "" {
		label = host
	}

	ifacesRaw := strings.TrimSpace(b.interfacesInput.Value())
	if ifacesRaw == "" {
		b.err = "At least one interface is required"
		return false
	}

	var ifaces []string
	for _, part := range strings.Split(ifacesRaw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			ifaces = append(ifaces, trimmed)
		}
	}
	if len(ifaces) == 0 {
		b.err = "At least one interface is required"
		return false
	}

	t := dashboard.Target{
		Host:       host,
		Label:      label,
		Identity:   strings.TrimSpace(b.targetIdentity.Value()),
		Interfaces: ifaces,
	}

	b.targets = append(b.targets, t)
	b.err = ""
	return true
}

func (b BuilderView) updateStepTargets(msg tea.Msg) (BuilderView, tea.Cmd, BuilderAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			if b.addingTarget {
				// If we have targets already, leave the form and show the list
				if len(b.targets) > 0 {
					b.addingTarget = false
					b.err = ""
					return b, nil, BuilderActionNone
				}
				// No targets, go back to step 1
				b.err = ""
				b.step = StepName
				b.focusStep1(0)
				return b, nil, BuilderActionNone
			}
			// Not adding, go back to step 1
			b.err = ""
			b.step = StepName
			b.focusStep1(0)
			return b, nil, BuilderActionNone

		case msg.String() == "tab":
			if b.addingTarget {
				b.focusStep2(b.step2Focus + 1)
				return b, nil, BuilderActionNone
			}

		case msg.String() == "shift+tab":
			if b.addingTarget {
				b.focusStep2(b.step2Focus - 1)
				return b, nil, BuilderActionNone
			}

		case msg.String() == "enter":
			if b.addingTarget {
				// If not on last field, advance focus
				if b.step2Focus < step2Fields-1 {
					b.focusStep2(b.step2Focus + 1)
					return b, nil, BuilderActionNone
				}
				// On last field, commit this target and show list
				if b.commitCurrentTarget() {
					b.addingTarget = false
					b.targetCursor = len(b.targets) - 1
				}
				return b, nil, BuilderActionNone
			}
			// In list mode, proceed to review
			if len(b.targets) > 0 {
				b.err = ""
				b.step = StepReview
				return b, nil, BuilderActionNone
			}
			b.err = "Add at least one target"
			return b, nil, BuilderActionNone

		case msg.String() == "a":
			if !b.addingTarget {
				b.addingTarget = true
				b.resetTargetForm()
				return b, nil, BuilderActionNone
			}
			// If adding, pass to input
			var cmd tea.Cmd
			switch b.step2Focus {
			case 0:
				b.hostInput, cmd = b.hostInput.Update(msg)
			case 1:
				b.labelInput, cmd = b.labelInput.Update(msg)
			case 2:
				b.targetIdentity, cmd = b.targetIdentity.Update(msg)
			case 3:
				b.interfacesInput, cmd = b.interfacesInput.Update(msg)
			}
			return b, cmd, BuilderActionNone

		case msg.String() == "d":
			if !b.addingTarget && len(b.targets) > 0 {
				b.targets = append(b.targets[:b.targetCursor], b.targets[b.targetCursor+1:]...)
				if b.targetCursor >= len(b.targets) && b.targetCursor > 0 {
					b.targetCursor--
				}
				// If no targets left, go into add mode
				if len(b.targets) == 0 {
					b.addingTarget = true
					b.resetTargetForm()
				}
				return b, nil, BuilderActionNone
			}
			if b.addingTarget {
				var cmd tea.Cmd
				switch b.step2Focus {
				case 0:
					b.hostInput, cmd = b.hostInput.Update(msg)
				case 1:
					b.labelInput, cmd = b.labelInput.Update(msg)
				case 2:
					b.targetIdentity, cmd = b.targetIdentity.Update(msg)
				case 3:
					b.interfacesInput, cmd = b.interfacesInput.Update(msg)
				}
				return b, cmd, BuilderActionNone
			}

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if !b.addingTarget && b.targetCursor > 0 {
				b.targetCursor--
			}
			return b, nil, BuilderActionNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if !b.addingTarget && b.targetCursor < len(b.targets)-1 {
				b.targetCursor++
			}
			return b, nil, BuilderActionNone

		default:
			if b.addingTarget {
				var cmd tea.Cmd
				switch b.step2Focus {
				case 0:
					b.hostInput, cmd = b.hostInput.Update(msg)
				case 1:
					b.labelInput, cmd = b.labelInput.Update(msg)
				case 2:
					b.targetIdentity, cmd = b.targetIdentity.Update(msg)
				case 3:
					b.interfacesInput, cmd = b.interfacesInput.Update(msg)
				}
				return b, cmd, BuilderActionNone
			}
		}
	}
	return b, nil, BuilderActionNone
}

func (b BuilderView) viewStepTargets() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base0D).
		Bold(true)
	stepStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base04)
	labelStyle := b.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base0D).
		Bold(true)

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render("New Dashboard") + "  " + stepStyle.Render("Step 2 of 3 - Add Targets") + "\n")
	s.WriteString("\n")

	if b.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(b.theme.Base08)
		s.WriteString("  " + errStyle.Render(b.err) + "\n\n")
	}

	// Show existing targets table
	if len(b.targets) > 0 {
		s.WriteString(b.renderTargetTable())
		s.WriteString("\n")
	}

	// Show the add-target form if in adding mode
	if b.addingTarget {
		subTitle := lipgloss.NewStyle().
			Foreground(b.theme.Base0E).
			Bold(true)
		s.WriteString("  " + subTitle.Render("Add Target") + "\n\n")

		type field struct {
			label string
			view  string
		}
		fields := []field{
			{"Host", b.hostInput.View()},
			{"Label", b.labelInput.View()},
			{"Identity", b.targetIdentity.View()},
			{"Interfaces", b.interfacesInput.View()},
		}

		for i, f := range fields {
			isFocused := i == b.step2Focus
			indicator := "  "
			lbl := labelStyle
			if isFocused {
				indicatorStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)
				indicator = indicatorStyle.Render("> ")
				lbl = activeLabelStyle
			}
			s.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(f.label+":", 18)), f.view))
		}
	}

	s.WriteString("\n")
	s.WriteString("  " + b.renderStep2Help() + "\n")

	return s.String()
}

func (b BuilderView) renderTargetTable() string {
	var s strings.Builder

	headerStyle := b.sty.TableHeader
	sepStyle := lipgloss.NewStyle().Foreground(b.theme.Base03)

	hostW := 20
	labelW := 16
	ifacesW := 30

	// Find max widths
	for _, t := range b.targets {
		if len(t.Host)+2 > hostW {
			hostW = len(t.Host) + 2
		}
		if len(t.Label)+2 > labelW {
			labelW = len(t.Label) + 2
		}
	}
	if hostW > 30 {
		hostW = 30
	}
	if labelW > 24 {
		labelW = 24
	}

	header := fmt.Sprintf("  %s%s%s%s",
		"  ",
		headerStyle.Render(padRight("Host", hostW)),
		headerStyle.Render(padRight("Label", labelW)),
		headerStyle.Render(padRight("Interfaces", ifacesW)),
	)
	s.WriteString(header + "\n")

	totalW := hostW + labelW + ifacesW + 2
	s.WriteString("  " + "  " + sepStyle.Render(strings.Repeat("-", totalW)) + "\n")

	for i, t := range b.targets {
		selected := !b.addingTarget && i == b.targetCursor

		cursor := "  "
		if selected {
			cursorStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)
			cursor = cursorStyle.Render("> ")
		}

		rowStyle := b.sty.TableRow
		if selected {
			rowStyle = b.sty.TableRowSel
		}

		ifaceStr := strings.Join(t.Interfaces, ", ")
		line := fmt.Sprintf("  %s%s%s%s",
			cursor,
			rowStyle.Render(padRight(truncate(t.Host, hostW-1), hostW)),
			rowStyle.Render(padRight(truncate(t.Label, labelW-1), labelW)),
			rowStyle.Render(padRight(truncate(ifaceStr, ifacesW-1), ifacesW)),
		)
		s.WriteString(line + "\n")
	}

	return s.String()
}

func (b BuilderView) renderStep2Help() string {
	helpStyle := lipgloss.NewStyle().Foreground(b.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)

	if b.addingTarget {
		return helpStyle.Render(fmt.Sprintf(
			"%s/%s navigate  %s next/commit  %s cancel",
			keyStyle.Render("[tab]"),
			keyStyle.Render("[shift+tab]"),
			keyStyle.Render("[enter]"),
			keyStyle.Render("[esc]"),
		))
	}

	return helpStyle.Render(fmt.Sprintf(
		"%s add  %s delete  %s/%s select  %s proceed  %s back",
		keyStyle.Render("[a]"),
		keyStyle.Render("[d]"),
		keyStyle.Render("[up]"),
		keyStyle.Render("[down]"),
		keyStyle.Render("[enter]"),
		keyStyle.Render("[esc]"),
	))
}

// --- Step 3: Review & Save ---

func (b BuilderView) updateStepReview(msg tea.Msg) (BuilderView, tea.Cmd, BuilderAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			// Go back to targets step
			b.step = StepTargets
			b.addingTarget = false
			b.err = ""
			return b, nil, BuilderActionNone

		case msg.String() == "enter":
			// Save the dashboard
			return b.saveDashboard()
		}
	}
	return b, nil, BuilderActionNone
}

func (b BuilderView) saveDashboard() (BuilderView, tea.Cmd, BuilderAction) {
	name := strings.TrimSpace(b.nameInput.Value())
	if name == "" {
		b.err = "Dashboard name is required"
		return b, nil, BuilderActionNone
	}

	intervalStr := strings.TrimSpace(b.intervalInput.Value())
	if intervalStr == "" {
		intervalStr = "10s"
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		b.err = fmt.Sprintf("Invalid interval: %v", err)
		return b, nil, BuilderActionNone
	}

	defaultIdentity := strings.TrimSpace(b.identityInput.Value())

	// Inherit default identity for targets without an explicit override
	targets := make([]dashboard.Target, len(b.targets))
	copy(targets, b.targets)
	for i := range targets {
		if targets[i].Identity == "" {
			targets[i].Identity = defaultIdentity
		}
		if targets[i].Port == 0 {
			targets[i].Port = 161
		}
	}

	dash := &dashboard.Dashboard{
		Name:            name,
		DefaultIdentity: defaultIdentity,
		Interval:        interval,
		MaxHistory:      360,
		Groups: []dashboard.Group{
			{
				Name:    "Default",
				Targets: targets,
			},
		},
	}

	dashDir, err := config.GetDashboardsDir()
	if err != nil {
		b.err = fmt.Sprintf("Failed to get dashboards dir: %v", err)
		return b, nil, BuilderActionNone
	}

	slug := slugify(name)
	path := filepath.Join(dashDir, slug+".toml")

	if err := config.EnsureDirs(); err != nil {
		b.err = fmt.Sprintf("Failed to create directories: %v", err)
		return b, nil, BuilderActionNone
	}

	if err := dashboard.SaveDashboard(dash, path); err != nil {
		b.err = fmt.Sprintf("Failed to save: %v", err)
		return b, nil, BuilderActionNone
	}

	b.SavedPath = path
	return b, nil, BuilderActionSave
}

func (b BuilderView) viewStepReview() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base0D).
		Bold(true)
	stepStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base04)
	labelStyle := b.sty.FormLabel
	valStyle := lipgloss.NewStyle().
		Foreground(b.theme.Base06)
	subTitle := lipgloss.NewStyle().
		Foreground(b.theme.Base0E).
		Bold(true)

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString("  " + titleStyle.Render("New Dashboard") + "  " + stepStyle.Render("Step 3 of 3 - Review") + "\n")
	s.WriteString("\n")

	if b.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(b.theme.Base08)
		s.WriteString("  " + errStyle.Render(b.err) + "\n\n")
	}

	name := strings.TrimSpace(b.nameInput.Value())
	ident := strings.TrimSpace(b.identityInput.Value())
	if ident == "" {
		ident = "(none)"
	}
	interval := strings.TrimSpace(b.intervalInput.Value())
	if interval == "" {
		interval = "10s"
	}

	s.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render(padRight("Name:", 18)), valStyle.Render(name)))
	s.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render(padRight("Identity:", 18)), valStyle.Render(ident)))
	s.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render(padRight("Poll Interval:", 18)), valStyle.Render(interval)))
	s.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render(padRight("Targets:", 18)), valStyle.Render(fmt.Sprintf("%d", len(b.targets)))))
	s.WriteString("\n")

	s.WriteString("  " + subTitle.Render("Targets (Group: Default)") + "\n\n")

	for i, t := range b.targets {
		idStr := t.Identity
		if idStr == "" {
			idStr = "(default)"
		}
		ifStr := strings.Join(t.Interfaces, ", ")
		s.WriteString(fmt.Sprintf("    %d. %s (%s)\n",
			i+1,
			valStyle.Render(t.Label),
			labelStyle.Render(t.Host),
		))
		s.WriteString(fmt.Sprintf("       Identity: %s  Interfaces: %s\n",
			labelStyle.Render(idStr),
			valStyle.Render(ifStr),
		))
	}

	slug := slugify(name)
	s.WriteString("\n")
	saveHint := lipgloss.NewStyle().Foreground(b.theme.Base04)
	s.WriteString("  " + saveHint.Render(fmt.Sprintf("Will save to: %s.toml", slug)) + "\n")

	s.WriteString("\n")
	s.WriteString("  " + b.renderStep3Help() + "\n")

	return s.String()
}

func (b BuilderView) renderStep3Help() string {
	helpStyle := lipgloss.NewStyle().Foreground(b.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(b.theme.Base0D).Bold(true)
	return helpStyle.Render(fmt.Sprintf(
		"%s save  %s back",
		keyStyle.Render("[enter]"),
		keyStyle.Render("[esc]"),
	))
}

// slugify converts a name to a URL-safe slug for use as a filename.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	// Remove characters that are not alphanumeric or hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
