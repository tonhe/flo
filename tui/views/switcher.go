package views

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// SwitcherAction describes what the app should do after a switcher key press.
type SwitcherAction int

const (
	// ActionNone means no action needed.
	ActionNone SwitcherAction = iota
	// ActionClose means the user wants to dismiss the switcher.
	ActionClose
	// ActionSwitch means the user selected a dashboard to switch to.
	ActionSwitch
	// ActionStop means the user wants to stop the selected engine.
	ActionStop
	// ActionNew means the user wants to create a new dashboard.
	ActionNew
	// ActionEdit means the user wants to edit the selected dashboard.
	ActionEdit
)

// SwitcherItem represents a single dashboard entry in the switcher list.
type SwitcherItem struct {
	Name     string
	FilePath string
	Running  bool
	Info     engine.EngineInfo
}

// SwitcherView is a modal overlay that lists dashboards and lets the user
// switch between them or start/stop engines.
type SwitcherView struct {
	theme  styles.Theme
	sty    *styles.Styles
	items  []SwitcherItem
	cursor int
	width  int
	height int
}

// NewSwitcherView creates a new SwitcherView with the given theme.
func NewSwitcherView(theme styles.Theme) SwitcherView {
	return SwitcherView{
		theme: theme,
		sty:   styles.NewStyles(theme),
	}
}

// Refresh scans the dashboards directory and checks which engines are running.
func (v *SwitcherView) Refresh(dashDir string, mgr *engine.Manager) {
	v.items = nil

	names, err := dashboard.ListDashboards(dashDir)
	if err != nil {
		return
	}

	runningEngines := mgr.ListEngines()
	runningMap := make(map[string]engine.EngineInfo, len(runningEngines))
	for _, info := range runningEngines {
		runningMap[info.Name] = info
	}

	for _, name := range names {
		item := SwitcherItem{
			Name:     name,
			FilePath: filepath.Join(dashDir, name+".toml"),
		}
		if info, ok := runningMap[name]; ok {
			item.Running = true
			item.Info = info
		}
		v.items = append(v.items, item)
	}

	// Clamp cursor
	if v.cursor >= len(v.items) {
		v.cursor = len(v.items) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

// SetSize updates the available dimensions for the overlay.
func (v *SwitcherView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SelectedItem returns the currently highlighted item, or nil if the list is
// empty.
func (v *SwitcherView) SelectedItem() *SwitcherItem {
	if len(v.items) == 0 {
		return nil
	}
	return &v.items[v.cursor]
}

// Update handles key messages for the switcher overlay.
func (v SwitcherView) Update(msg tea.Msg) (SwitcherView, tea.Cmd, SwitcherAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return v, nil, ActionClose

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if v.cursor > 0 {
				v.cursor--
			}
			return v, nil, ActionNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if v.cursor < len(v.items)-1 {
				v.cursor++
			}
			return v, nil, ActionNone

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			if len(v.items) > 0 {
				return v, nil, ActionSwitch
			}
			return v, nil, ActionNone

		case msg.String() == "x":
			if len(v.items) > 0 && v.items[v.cursor].Running {
				return v, nil, ActionStop
			}
			return v, nil, ActionNone

		case msg.String() == "n":
			return v, nil, ActionNew

		case msg.String() == "e":
			if len(v.items) > 0 {
				return v, nil, ActionEdit
			}
			return v, nil, ActionNone
		}
	}
	return v, nil, ActionNone
}

// View renders the switcher as a centered modal box.
func (v SwitcherView) View() string {
	// Calculate modal dimensions
	modalWidth := 44
	if v.width > 60 {
		modalWidth = v.width / 2
		if modalWidth > 60 {
			modalWidth = 60
		}
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	// Inner content width (subtract border + padding: 2 border + 4 padding)
	innerWidth := modalWidth - 6

	var lines []string

	if len(v.items) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
		lines = append(lines, dimStyle.Render("No dashboards found."))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Press [n] to create one."))
	} else {
		for i, item := range v.items {
			line := v.renderItem(item, i == v.cursor, innerWidth)
			lines = append(lines, line)
		}
	}

	// Help line at the bottom
	helpStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	helpKeyStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
	help := fmt.Sprintf(
		"%s:switch  %s:new  %s:edit  %s:stop  %s:close",
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("n"),
		helpKeyStyle.Render("e"),
		helpKeyStyle.Render("x"),
		helpKeyStyle.Render("esc"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(lines, "\n"),
		"",
		helpStyle.Render(help),
	)

	// Modal box with rounded border
	modal := v.sty.ModalBorder.
		Width(innerWidth).
		Render(content)

	// Title rendered into the border top
	title := v.sty.ModalTitle.Render(" Dashboards ")
	modal = lipgloss.JoinVertical(lipgloss.Left, modal)

	// Place the title over the top border
	modalLines := strings.Split(modal, "\n")
	if len(modalLines) > 0 {
		borderLine := modalLines[0]
		// Insert the title after the first border character
		if len(borderLine) > 2 {
			runes := []rune(borderLine)
			titleRunes := []rune(title)
			// Place title starting at position 2 (after corner + one border char)
			insertPos := 2
			if insertPos+len(titleRunes) < len(runes) {
				combined := make([]rune, 0, len(runes))
				combined = append(combined, runes[:insertPos]...)
				combined = append(combined, titleRunes...)
				combined = append(combined, runes[insertPos+len(titleRunes):]...)
				modalLines[0] = string(combined)
			}
		}
		modal = strings.Join(modalLines, "\n")
	}

	// Center the modal in the available space
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, modal)
}

// renderItem renders a single dashboard item line.
func (v SwitcherView) renderItem(item SwitcherItem, selected bool, width int) string {
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = "> "
	}

	cursorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)

	// Name
	nameStyle := lipgloss.NewStyle().Foreground(v.theme.Base05)
	if selected {
		nameStyle = nameStyle.Foreground(v.theme.Base06).Bold(true)
	}

	// Status indicator and text
	var statusStr string
	if item.Running {
		dotStyle := lipgloss.NewStyle().Foreground(v.theme.Base0B)
		liveStyle := lipgloss.NewStyle().Foreground(v.theme.Base0B)
		pollStr := fmt.Sprintf("(%d)", item.Info.PollCount)
		pollStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
		statusStr = fmt.Sprintf(
			"%s %s  %s",
			dotStyle.Render("*"),
			liveStyle.Render("LIVE"),
			pollStyle.Render(pollStr),
		)
	} else {
		dotStyle := lipgloss.NewStyle().Foreground(v.theme.Base03)
		stoppedStyle := lipgloss.NewStyle().Foreground(v.theme.Base03)
		statusStr = fmt.Sprintf(
			"%s %s",
			dotStyle.Render("o"),
			stoppedStyle.Render("stopped"),
		)
	}

	// Build the line: cursor + name + padding + status
	nameText := nameStyle.Render(item.Name)
	cursorText := cursorStyle.Render(cursor)

	// Calculate padding to right-align the status
	nameLen := len(cursor) + len(item.Name)
	// Approximate status plain text length
	statusPlainLen := 0
	if item.Running {
		statusPlainLen = len(fmt.Sprintf("* LIVE  (%d)", item.Info.PollCount))
	} else {
		statusPlainLen = len("o stopped")
	}

	padLen := width - nameLen - statusPlainLen
	if padLen < 2 {
		padLen = 2
	}
	padding := strings.Repeat(" ", padLen)

	return cursorText + nameText + padding + statusStr
}
