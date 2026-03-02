package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// ThemePickerAction describes the result of a ThemePickerModel update.
type ThemePickerAction int

const (
	// ThemePickerNone means no action; the picker remains open.
	ThemePickerNone ThemePickerAction = iota
	// ThemePickerSelected means the user confirmed a theme selection.
	ThemePickerSelected
	// ThemePickerCancelled means the user dismissed the picker without selecting.
	ThemePickerCancelled
)

// ThemePickerModel is a full-screen two-panel theme browser. The left panel
// shows a scrollable list of all available themes; the right panel shows a
// sample dashboard preview rendered in the currently highlighted theme's colors.
type ThemePickerModel struct {
	themes       []string // sorted theme slugs
	cursor       int      // index of highlighted theme
	scrollOffset int      // first visible index in the list
	initialIndex int      // starting index (for reference by caller)

	width  int
	height int
}

// NewThemePickerModel creates a new ThemePickerModel with the cursor positioned
// at the theme identified by currentSlug. If the slug is not found, the cursor
// starts at index 0.
func NewThemePickerModel(currentSlug string) ThemePickerModel {
	themes := styles.ListThemes()
	idx := styles.GetThemeIndex(currentSlug)
	if idx < 0 {
		idx = 0
	}
	return ThemePickerModel{
		themes:       themes,
		cursor:       idx,
		initialIndex: idx,
	}
}

// SetSize updates the available terminal dimensions for layout.
func (m *ThemePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// PreviewTheme returns the theme that is currently highlighted in the list.
func (m ThemePickerModel) PreviewTheme() styles.Theme {
	if m.cursor >= 0 && m.cursor < len(m.themes) {
		t := styles.GetThemeByName(m.themes[m.cursor])
		if t != nil {
			return *t
		}
	}
	return styles.DefaultTheme
}

// SelectedSlug returns the slug of the currently highlighted theme.
func (m ThemePickerModel) SelectedSlug() string {
	if m.cursor >= 0 && m.cursor < len(m.themes) {
		return m.themes[m.cursor]
	}
	return ""
}

// Update handles key messages and returns the updated model, a command, and
// the resulting ThemePickerAction.
func (m ThemePickerModel) Update(msg tea.Msg) (ThemePickerModel, tea.Cmd, ThemePickerAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return m, nil, ThemePickerCancelled

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil, ThemePickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if m.cursor < len(m.themes)-1 {
				m.cursor++
			}
			return m, nil, ThemePickerNone

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			return m, nil, ThemePickerSelected
		}
	}
	return m, nil, ThemePickerNone
}

// View renders the full-screen two-panel theme picker.
func (m ThemePickerModel) View() string {
	theme := m.PreviewTheme()
	bg := theme.Base00

	// Compute available content area (reserve 2 lines for title, 1 blank, 1 help, 1 bottom padding).
	contentHeight := m.height - 5
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Compute panel widths. Left panel gets ~40%, right panel gets the rest,
	// with a 1-char separator column.
	leftWidth := m.width * 2 / 5
	if leftWidth < 26 {
		leftWidth = 26
	}
	separatorWidth := 3 // " | "
	rightWidth := m.width - leftWidth - separatorWidth
	if rightWidth < 30 {
		rightWidth = 30
	}

	// Adjust scroll offset so the cursor is always visible.
	visibleRows := contentHeight
	// Reserve rows for scroll indicators if needed.
	listLen := len(m.themes)
	m.adjustScroll(visibleRows, listLen)

	// Build the left panel (theme list).
	leftLines := m.renderThemeList(theme, leftWidth, visibleRows, listLen)

	// Build the right panel (preview).
	rightLines := m.renderPreviewPanel(theme, rightWidth, visibleRows)

	// Pad both panels to the same height.
	for len(leftLines) < visibleRows {
		leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
	}
	for len(rightLines) < visibleRows {
		rightLines = append(rightLines, strings.Repeat(" ", rightWidth))
	}

	// Compose the two panels side by side.
	sepStyle := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg)
	sep := sepStyle.Render(" \u2502 ")

	var panelLines []string
	for i := 0; i < visibleRows; i++ {
		left := leftLines[i]
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		panelLines = append(panelLines, left+sep+right)
	}

	// Title line: "  Select Theme  (cursor/total)"
	titleStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	titleLine := titleStyle.Render("  Select Theme") + countStyle.Render(fmt.Sprintf("  (%d/%d)", m.cursor+1, len(m.themes)))

	// Help line.
	helpStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	helpKeyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	helpLine := helpStyle.Render("  ") +
		helpKeyStyle.Render("\u2191") + helpStyle.Render("/") + helpKeyStyle.Render("\u2193") + helpStyle.Render(" select   ") +
		helpKeyStyle.Render("enter") + helpStyle.Render(" confirm   ") +
		helpKeyStyle.Render("esc") + helpStyle.Render(" cancel")

	// Assemble the full view.
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		"",
		strings.Join(panelLines, "\n"),
		"",
		helpLine,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content,
		lipgloss.WithWhitespaceBackground(bg))
}

// adjustScroll ensures the cursor is visible within the scroll window.
func (m *ThemePickerModel) adjustScroll(visibleRows, listLen int) {
	// Scroll down if cursor is below the visible window.
	for {
		topReserve := 0
		if m.scrollOffset > 0 {
			topReserve = 1
		}
		bottomReserve := 0
		if m.scrollOffset+visibleRows < listLen {
			bottomReserve = 1
		}
		usable := visibleRows - topReserve - bottomReserve
		if usable < 1 {
			usable = 1
		}
		if m.cursor >= m.scrollOffset+topReserve+usable {
			m.scrollOffset++
		} else {
			break
		}
	}

	// Scroll up if cursor is above the visible window.
	for {
		topReserve := 0
		if m.scrollOffset > 0 {
			topReserve = 1
		}
		if m.cursor < m.scrollOffset+topReserve {
			m.scrollOffset--
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
				break
			}
		} else {
			break
		}
	}
}

// renderThemeList builds the left panel lines showing the scrollable theme list.
func (m ThemePickerModel) renderThemeList(theme styles.Theme, width, visibleRows, listLen int) []string {
	bg := theme.Base00
	normalStyle := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg)
	selectedStyle := lipgloss.NewStyle().Foreground(theme.Base06).Background(bg).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	arrowStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)

	showTopIndicator := m.scrollOffset > 0
	showBottomIndicator := m.scrollOffset+visibleRows < listLen

	var lines []string

	// Top scroll indicator.
	if showTopIndicator {
		indicator := dimStyle.Render(themePickerPadRight("    \u25b2 more", width))
		lines = append(lines, indicator)
	}

	// Compute how many theme rows we can show.
	usableRows := visibleRows
	if showTopIndicator {
		usableRows--
	}
	if showBottomIndicator {
		usableRows--
	}
	if usableRows < 1 {
		usableRows = 1
	}

	startIdx := m.scrollOffset
	endIdx := startIdx + usableRows
	if endIdx > listLen {
		endIdx = listLen
	}

	for i := startIdx; i < endIdx; i++ {
		slug := m.themes[i]
		t := styles.GetThemeByName(slug)
		name := slug
		if t != nil {
			name = t.Name
		}

		isCursor := i == m.cursor

		prefix := "    "
		suffix := "   "
		if isCursor {
			prefix = cursorStyle.Render("  > ")
			suffix = arrowStyle.Render(" \u25c0 ")
		}

		// Compute the available name width (accounting for prefix and suffix visible widths).
		// prefix is 4 chars, suffix is 3 chars.
		nameWidth := width - 4 - 3
		if nameWidth < 10 {
			nameWidth = 10
		}

		displayName := name
		if len(displayName) > nameWidth {
			displayName = displayName[:nameWidth-1] + "\u2026"
		}

		var styledName string
		if isCursor {
			styledName = selectedStyle.Render(themePickerPadRight(displayName, nameWidth))
		} else {
			styledName = normalStyle.Render(themePickerPadRight(displayName, nameWidth))
		}

		line := prefix + styledName + suffix
		lines = append(lines, line)
	}

	// Bottom scroll indicator.
	if showBottomIndicator {
		indicator := dimStyle.Render(themePickerPadRight("    \u25bc more", width))
		lines = append(lines, indicator)
	}

	return lines
}

// renderPreviewPanel builds the right panel lines showing a sample dashboard
// rendered in the highlighted theme's colors.
func (m ThemePickerModel) renderPreviewPanel(theme styles.Theme, width, visibleRows int) []string {
	bg := theme.Base00

	sepStyle := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg)
	titleBg := lipgloss.NewStyle().
		Background(theme.Base01).
		Foreground(theme.Base05).
		Bold(true)
	titleAccent := lipgloss.NewStyle().
		Background(theme.Base01).
		Foreground(theme.Base0D).
		Bold(true)
	thStyle := lipgloss.NewStyle().
		Foreground(theme.Base0D).
		Background(bg).
		Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg)
	upStyle := lipgloss.NewStyle().Foreground(theme.Base0B).Background(bg)
	downStyle := lipgloss.NewStyle().Foreground(theme.Base08).Background(bg)
	warnStyle := lipgloss.NewStyle().Foreground(theme.Base0A).Background(bg)
	utilLow := lipgloss.NewStyle().Foreground(theme.Base0B).Background(bg)
	utilHigh := lipgloss.NewStyle().Foreground(theme.Base08).Background(bg)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	bgStyle := lipgloss.NewStyle().Background(bg)

	var lines []string

	// Header bar: "flo - Sample Dashboard" with Base01 background.
	headerText := titleAccent.Render("flo") + titleBg.Render(" - Sample Dashboard")
	headerPad := width - lipgloss.Width(headerText)
	if headerPad < 0 {
		headerPad = 0
	}
	headerLine := headerText + titleBg.Render(strings.Repeat(" ", headerPad))
	lines = append(lines, headerLine)

	// Separator.
	lines = append(lines, sepStyle.Render(strings.Repeat("\u2500", width)))

	// Table header.
	ifaceW := 16
	statusW := 10
	utilW := 14
	tableHeader := " " +
		thStyle.Render(themePickerPadRight("Interface", ifaceW)) +
		thStyle.Render(themePickerPadRight("Status", statusW)) +
		thStyle.Render(themePickerPadRight("Utilization", utilW))
	lines = append(lines, tableHeader)

	// Sample data rows.
	sampleRows := []struct {
		name   string
		status string
		sStyle lipgloss.Style
		util   string
		uStyle lipgloss.Style
	}{
		{"Gi0/0/0", "Up", upStyle, "23%", utilLow},
		{"Gi0/0/1", "Down", downStyle, "0%", downStyle},
		{"Gi0/0/2", "Up", upStyle, "87%", utilHigh},
		{"Gi0/0/3", "Up", warnStyle, "45%", warnStyle},
	}

	for _, r := range sampleRows {
		row := " " +
			rowStyle.Render(themePickerPadRight(r.name, ifaceW)) +
			r.sStyle.Render(themePickerPadRight(r.status, statusW)) +
			r.uStyle.Render(themePickerPadRight(r.util, utilW))
		lines = append(lines, row)
	}

	// Blank line.
	lines = append(lines, bgStyle.Render(strings.Repeat(" ", width)))

	// Color swatches.
	colorPairs := []struct {
		name  string
		color lipgloss.Color
	}{
		{"red", theme.Base08},
		{"org", theme.Base09},
		{"yel", theme.Base0A},
		{"grn", theme.Base0B},
		{"cyn", theme.Base0C},
		{"blu", theme.Base0D},
		{"mag", theme.Base0E},
	}

	swatchLine := dimStyle.Render(" Colors: ")
	for i, cp := range colorPairs {
		cs := lipgloss.NewStyle().Foreground(cp.color).Background(bg)
		swatchLine += cs.Render(cp.name)
		if i < len(colorPairs)-1 {
			swatchLine += bgStyle.Render(" ")
		}
	}
	lines = append(lines, swatchLine)

	// Bottom separator.
	lines = append(lines, sepStyle.Render(strings.Repeat("\u2500", width)))

	return lines
}

// themePickerPadRight right-pads s with spaces to the given width. This is
// local to the theme picker to avoid conflicts with other file-local helpers.
func themePickerPadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
