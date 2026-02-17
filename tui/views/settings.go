package views

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// SettingsAction describes what the app should do after a settings update.
type SettingsAction int

const (
	// SettingsNone means continue in the settings view.
	SettingsNone SettingsAction = iota
	// SettingsClose means the user cancelled without saving.
	SettingsClose
	// SettingsSaved means the config was saved; the app should apply changes.
	SettingsSaved
)

// Settings field indices.
const (
	settingsFieldTheme    = 0
	settingsFieldIdentity = 1
	settingsFieldInterval = 2
	settingsFieldHistory  = 3
	settingsFieldCount    = 4
)

// SettingsView is a full-screen settings editor with a live theme preview.
type SettingsView struct {
	theme  styles.Theme
	sty    *styles.Styles
	config *config.Config

	themeIndex int // index into styles.ListThemes()
	cursor     int // which setting row is focused

	width  int
	height int

	// Editable text fields
	identityInput textinput.Model
	intervalInput textinput.Model
	historyInput  textinput.Model

	// State
	changed   bool
	err       string
	SavedTheme string // theme slug after save, so the app can apply it
}

// NewSettingsView creates a fresh SettingsView populated from the current config.
func NewSettingsView(theme styles.Theme, cfg *config.Config) SettingsView {
	sty := styles.NewStyles(theme)

	themeIdx := styles.GetThemeIndex(cfg.Theme)
	if themeIdx < 0 {
		themeIdx = 0
	}

	identityInput := textinput.New()
	identityInput.Placeholder = "default identity"
	identityInput.CharLimit = 64
	identityInput.Width = 40
	identityInput.SetValue(cfg.DefaultIdentity)

	intervalInput := textinput.New()
	intervalInput.Placeholder = "10s"
	intervalInput.CharLimit = 16
	intervalInput.Width = 40
	intervalInput.SetValue(cfg.PollInterval.String())

	historyInput := textinput.New()
	historyInput.Placeholder = "360"
	historyInput.CharLimit = 8
	historyInput.Width = 40
	historyInput.SetValue(strconv.Itoa(cfg.MaxHistory))

	return SettingsView{
		theme:         theme,
		sty:           sty,
		config:        cfg,
		themeIndex:    themeIdx,
		cursor:        0,
		identityInput: identityInput,
		intervalInput: intervalInput,
		historyInput:  historyInput,
	}
}

// SetSize updates the available dimensions for the settings view.
func (s *SettingsView) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// selectedThemeSlug returns the slug of the currently selected theme.
func (s SettingsView) selectedThemeSlug() string {
	themes := styles.ListThemes()
	if s.themeIndex >= 0 && s.themeIndex < len(themes) {
		return themes[s.themeIndex]
	}
	return ""
}

// selectedTheme returns the Theme struct for the currently selected theme.
func (s SettingsView) selectedTheme() styles.Theme {
	t := styles.GetThemeByIndex(s.themeIndex)
	if t != nil {
		return *t
	}
	return styles.DefaultTheme
}

// focusInput blurs all inputs and focuses the one at the cursor position.
func (s *SettingsView) focusInput() {
	s.identityInput.Blur()
	s.intervalInput.Blur()
	s.historyInput.Blur()

	switch s.cursor {
	case settingsFieldIdentity:
		s.identityInput.Focus()
	case settingsFieldInterval:
		s.intervalInput.Focus()
	case settingsFieldHistory:
		s.historyInput.Focus()
	}
}

// Update handles messages for the settings view.
func (s SettingsView) Update(msg tea.Msg) (SettingsView, tea.Cmd, SettingsAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return s, nil, SettingsClose

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			return s.save()

		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if s.cursor > 0 {
				s.cursor--
				s.focusInput()
			}
			return s, nil, SettingsNone

		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if s.cursor < settingsFieldCount-1 {
				s.cursor++
				s.focusInput()
			}
			return s, nil, SettingsNone

		case msg.String() == "tab":
			s.cursor++
			if s.cursor >= settingsFieldCount {
				s.cursor = 0
			}
			s.focusInput()
			return s, nil, SettingsNone

		case msg.String() == "shift+tab":
			s.cursor--
			if s.cursor < 0 {
				s.cursor = settingsFieldCount - 1
			}
			s.focusInput()
			return s, nil, SettingsNone

		case key.Matches(msg, keys.DefaultKeyMap.Left):
			if s.cursor == settingsFieldTheme {
				themes := styles.ListThemes()
				s.themeIndex--
				if s.themeIndex < 0 {
					s.themeIndex = len(themes) - 1
				}
				// Update preview theme and styles
				s.theme = s.selectedTheme()
				s.sty = styles.NewStyles(s.theme)
				s.changed = true
				return s, nil, SettingsNone
			}
			// For text fields, pass through to the input
			return s.updateTextInput(msg)

		case key.Matches(msg, keys.DefaultKeyMap.Right):
			if s.cursor == settingsFieldTheme {
				themes := styles.ListThemes()
				s.themeIndex++
				if s.themeIndex >= len(themes) {
					s.themeIndex = 0
				}
				// Update preview theme and styles
				s.theme = s.selectedTheme()
				s.sty = styles.NewStyles(s.theme)
				s.changed = true
				return s, nil, SettingsNone
			}
			// For text fields, pass through to the input
			return s.updateTextInput(msg)

		default:
			// Pass keys to the focused text input
			return s.updateTextInput(msg)
		}
	}
	return s, nil, SettingsNone
}

// updateTextInput dispatches a key message to the currently focused text input.
func (s SettingsView) updateTextInput(msg tea.Msg) (SettingsView, tea.Cmd, SettingsAction) {
	var cmd tea.Cmd
	switch s.cursor {
	case settingsFieldIdentity:
		s.identityInput, cmd = s.identityInput.Update(msg)
	case settingsFieldInterval:
		s.intervalInput, cmd = s.intervalInput.Update(msg)
	case settingsFieldHistory:
		s.historyInput, cmd = s.historyInput.Update(msg)
	}
	return s, cmd, SettingsNone
}

// save validates and persists the config to disk.
func (s SettingsView) save() (SettingsView, tea.Cmd, SettingsAction) {
	// Validate poll interval
	intervalStr := strings.TrimSpace(s.intervalInput.Value())
	if intervalStr == "" {
		intervalStr = "10s"
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		s.err = fmt.Sprintf("Invalid poll interval: %v", err)
		return s, nil, SettingsNone
	}
	if interval < time.Second {
		s.err = "Poll interval must be at least 1s"
		return s, nil, SettingsNone
	}

	// Validate max history
	historyStr := strings.TrimSpace(s.historyInput.Value())
	if historyStr == "" {
		historyStr = "360"
	}
	maxHistory, err := strconv.Atoi(historyStr)
	if err != nil || maxHistory < 1 {
		s.err = "Max history must be a positive integer"
		return s, nil, SettingsNone
	}

	// Apply values to config
	s.config.Theme = s.selectedThemeSlug()
	s.config.DefaultIdentity = strings.TrimSpace(s.identityInput.Value())
	s.config.PollInterval = interval
	s.config.MaxHistory = maxHistory

	// Save to disk
	cfgDir, err := config.GetConfigDir()
	if err != nil {
		s.err = fmt.Sprintf("Failed to get config dir: %v", err)
		return s, nil, SettingsNone
	}
	if err := config.EnsureDirs(); err != nil {
		s.err = fmt.Sprintf("Failed to create directories: %v", err)
		return s, nil, SettingsNone
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	if err := config.SaveConfig(s.config, cfgPath); err != nil {
		s.err = fmt.Sprintf("Failed to save config: %v", err)
		return s, nil, SettingsNone
	}

	s.SavedTheme = s.config.Theme
	s.err = ""
	return s, nil, SettingsSaved
}

// View renders the settings screen.
func (s SettingsView) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(s.theme.Base0D).
		Bold(true)
	labelStyle := s.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().
		Foreground(s.theme.Base0D).
		Bold(true)
	valStyle := lipgloss.NewStyle().
		Foreground(s.theme.Base06)

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Settings") + "\n")
	b.WriteString("\n")

	if s.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(s.theme.Base08)
		b.WriteString("  " + errStyle.Render(s.err) + "\n\n")
	}

	// Theme row (cycle with left/right)
	themeSlug := s.selectedThemeSlug()
	themeName := themeSlug
	if t := styles.GetThemeByName(themeSlug); t != nil {
		themeName = t.Name
	}
	themeCount := styles.GetThemeCount()
	themeDisplay := fmt.Sprintf("< %s >  (%d/%d)", themeName, s.themeIndex+1, themeCount)

	type settingsRow struct {
		label   string
		display string
		isInput bool
		input   string
	}

	rows := []settingsRow{
		{"Theme", themeDisplay, false, ""},
		{"Default Identity", "", true, s.identityInput.View()},
		{"Poll Interval", "", true, s.intervalInput.View()},
		{"Max History", "", true, s.historyInput.View()},
	}

	for i, row := range rows {
		isFocused := i == s.cursor
		indicator := "  "
		lbl := labelStyle
		if isFocused {
			indicatorStyle := lipgloss.NewStyle().Foreground(s.theme.Base0D).Bold(true)
			indicator = indicatorStyle.Render("> ")
			lbl = activeLabelStyle
		}

		label := lbl.Render(padRight(row.label+":", 20))
		if row.isInput {
			b.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, label, row.input))
		} else {
			b.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, label, valStyle.Render(row.display)))
		}
	}

	// Theme preview
	b.WriteString("\n")
	b.WriteString(s.renderThemePreview())

	// Help line
	b.WriteString("\n")
	b.WriteString("  " + s.renderHelp() + "\n")

	return b.String()
}

// renderThemePreview renders a small preview panel showing the selected theme's colors.
func (s SettingsView) renderThemePreview() string {
	previewTheme := s.selectedTheme()

	sepStyle := lipgloss.NewStyle().Foreground(previewTheme.Base03)
	titleStyle := lipgloss.NewStyle().Foreground(previewTheme.Base0D).Bold(true)

	previewWidth := 56
	if s.width > 0 && s.width-6 < previewWidth {
		previewWidth = s.width - 6
	}
	if previewWidth < 30 {
		previewWidth = 30
	}

	var b strings.Builder

	// Preview header line
	label := " Theme Preview "
	dashCount := previewWidth - len(label)
	if dashCount < 2 {
		dashCount = 2
	}
	leftDash := dashCount / 2
	rightDash := dashCount - leftDash
	b.WriteString("  " + sepStyle.Render(strings.Repeat("-", leftDash)) + titleStyle.Render(label) + sepStyle.Render(strings.Repeat("-", rightDash)) + "\n")

	// Sample header bar
	headerBg := lipgloss.NewStyle().
		Background(previewTheme.Base01).
		Foreground(previewTheme.Base05).
		Bold(true).
		Padding(0, 1)
	headerTitle := lipgloss.NewStyle().
		Background(previewTheme.Base01).
		Foreground(previewTheme.Base0D).
		Bold(true)
	b.WriteString("  " + headerBg.Render(headerTitle.Render("flo")+" - Sample Dashboard"+strings.Repeat(" ", max(0, previewWidth-28))) + "\n")

	// Sample table header
	thStyle := lipgloss.NewStyle().
		Foreground(previewTheme.Base0D).
		Bold(true)
	b.WriteString("  " + fmt.Sprintf("  %s%s%s",
		thStyle.Render(padRight("Interface", 16)),
		thStyle.Render(padRight("Status", 10)),
		thStyle.Render(padRight("Utilization", 14)),
	) + "\n")

	// Sample rows
	rowStyle := lipgloss.NewStyle().Foreground(previewTheme.Base05)
	upStyle := lipgloss.NewStyle().Foreground(previewTheme.Base0B)
	downStyle := lipgloss.NewStyle().Foreground(previewTheme.Base08)
	warnStyle := lipgloss.NewStyle().Foreground(previewTheme.Base0A)
	utilLow := lipgloss.NewStyle().Foreground(previewTheme.Base0B)
	utilHigh := lipgloss.NewStyle().Foreground(previewTheme.Base08)

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
		b.WriteString("  " + fmt.Sprintf("  %s%s%s",
			rowStyle.Render(padRight(r.name, 16)),
			r.sStyle.Render(padRight(r.status, 10)),
			r.uStyle.Render(padRight(r.util, 14)),
		) + "\n")
	}

	// Color swatch row
	b.WriteString("\n")
	swatchLabel := lipgloss.NewStyle().Foreground(previewTheme.Base04)
	b.WriteString("  " + swatchLabel.Render("Colors: "))

	colorPairs := []struct {
		name  string
		color lipgloss.Color
	}{
		{"red", previewTheme.Base08},
		{"org", previewTheme.Base09},
		{"yel", previewTheme.Base0A},
		{"grn", previewTheme.Base0B},
		{"cyn", previewTheme.Base0C},
		{"blu", previewTheme.Base0D},
		{"mag", previewTheme.Base0E},
	}

	for _, cp := range colorPairs {
		cs := lipgloss.NewStyle().Foreground(cp.color)
		b.WriteString(cs.Render(cp.name) + " ")
	}
	b.WriteString("\n")

	// Bottom border
	b.WriteString("  " + sepStyle.Render(strings.Repeat("-", previewWidth)) + "\n")

	return b.String()
}

// renderHelp renders the help line for the settings view.
func (s SettingsView) renderHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(s.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(s.theme.Base0D).Bold(true)

	hint := ""
	if s.cursor == settingsFieldTheme {
		hint = fmt.Sprintf(
			"%s/%s cycle theme  %s/%s navigate  %s save  %s cancel",
			keyStyle.Render("[left]"),
			keyStyle.Render("[right]"),
			keyStyle.Render("[up]"),
			keyStyle.Render("[down]"),
			keyStyle.Render("[enter]"),
			keyStyle.Render("[esc]"),
		)
	} else {
		hint = fmt.Sprintf(
			"%s/%s navigate  %s save  %s cancel",
			keyStyle.Render("[up]"),
			keyStyle.Render("[down]"),
			keyStyle.Render("[enter]"),
			keyStyle.Render("[esc]"),
		)
	}

	return helpStyle.Render(hint)
}

