package views

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui/components"
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
	// SettingsManageIdentities means the user wants to open the identity manager.
	SettingsManageIdentities
)

// Settings field indices.
const (
	settingsFieldTheme      = 0
	settingsFieldIdentity   = 1
	settingsFieldHistory    = 2
	settingsFieldTimeFormat = 3
	settingsFieldManageIds  = 4
	settingsFieldCount      = 5
)

var timeFormats = []string{"relative", "absolute", "both"}

// SettingsView is a full-screen settings editor with a live theme preview.
type SettingsView struct {
	theme  styles.Theme
	sty    *styles.Styles
	config *config.Config

	themeIndex      int // index into styles.ListThemes()
	timeFormatIndex int // 0=relative, 1=absolute, 2=both
	cursor          int // which setting row is focused

	width  int
	height int

	// Editable text fields
	identityInput textinput.Model
	historyInput  textinput.Model

	// Identity picker
	showPicker bool
	picker     components.IdentityPickerModel
	provider   identity.Provider

	// Theme picker
	showThemePicker bool
	themePicker     components.ThemePickerModel

	// State
	changed      bool
	confirmClose bool   // show "save changes?" dialog
	err          string
	SavedTheme   string // theme slug after save, so the app can apply it
}

// NewSettingsView creates a fresh SettingsView populated from the current config.
func NewSettingsView(theme styles.Theme, cfg *config.Config, provider identity.Provider) SettingsView {
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

	historyInput := textinput.New()
	historyInput.Placeholder = "360"
	historyInput.CharLimit = 8
	historyInput.Width = 40
	historyInput.SetValue(strconv.Itoa(cfg.MaxHistory))

	timeFormatIdx := 0
	switch cfg.TimeFormat {
	case "absolute":
		timeFormatIdx = 1
	case "both":
		timeFormatIdx = 2
	}

	return SettingsView{
		theme:           theme,
		sty:             sty,
		config:          cfg,
		themeIndex:      themeIdx,
		timeFormatIndex: timeFormatIdx,
		cursor:          0,
		identityInput: identityInput,
		historyInput:  historyInput,
		provider:        provider,
	}
}

// SetSize updates the available dimensions for the settings view.
func (s *SettingsView) SetSize(width, height int) {
	s.width = width
	s.height = height
	if s.showThemePicker {
		s.themePicker.SetSize(width, height)
	}
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

// selectedTimeFormat returns the time format string for the currently selected index.
func (s SettingsView) selectedTimeFormat() string {
	if s.timeFormatIndex >= 0 && s.timeFormatIndex < len(timeFormats) {
		return timeFormats[s.timeFormatIndex]
	}
	return "relative"
}

// PreviewTheme returns the theme currently being previewed. When the
// full-screen picker is open this is the highlighted theme; otherwise
// it is the theme selected via left/right cycling.
func (s SettingsView) PreviewTheme() styles.Theme {
	if s.showThemePicker {
		return s.themePicker.PreviewTheme()
	}
	return s.theme
}

// focusInput blurs all inputs and focuses the one at the cursor position.
func (s *SettingsView) focusInput() {
	s.identityInput.Blur()
	s.historyInput.Blur()

	switch s.cursor {
	case settingsFieldIdentity:
		s.identityInput.Focus()
	case settingsFieldHistory:
		s.historyInput.Focus()
	// settingsFieldManageIds and settingsFieldTheme have no text input
	}
}

// hasChanges returns true if any setting differs from the persisted config.
func (s SettingsView) hasChanges() bool {
	if s.selectedThemeSlug() != s.config.Theme {
		return true
	}
	if strings.TrimSpace(s.identityInput.Value()) != s.config.DefaultIdentity {
		return true
	}
	if strings.TrimSpace(s.historyInput.Value()) != strconv.Itoa(s.config.MaxHistory) {
		return true
	}
	if s.selectedTimeFormat() != s.config.TimeFormat {
		return true
	}
	return false
}

// Update handles messages for the settings view.
func (s SettingsView) Update(msg tea.Msg) (SettingsView, tea.Cmd, SettingsAction) {
	// Save confirmation dialog intercepts all keys
	if s.confirmClose {
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			switch kmsg.String() {
			case "y":
				return s.save()
			case "n":
				return s, nil, SettingsClose
			case "esc":
				s.confirmClose = false
				return s, nil, SettingsNone
			}
		}
		return s, nil, SettingsNone
	}

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

	if s.showThemePicker {
		var cmd tea.Cmd
		var action components.ThemePickerAction
		s.themePicker, cmd, action = s.themePicker.Update(msg)
		switch action {
		case components.ThemePickerSelected:
			slug := s.themePicker.SelectedSlug()
			s.themeIndex = styles.GetThemeIndex(slug)
			if s.themeIndex < 0 {
				s.themeIndex = 0
			}
			s.theme = s.selectedTheme()
			s.sty = styles.NewStyles(s.theme)
			s.changed = true
			s.showThemePicker = false
		case components.ThemePickerCancelled:
			s.showThemePicker = false
		}
		return s, cmd, SettingsNone
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			if s.hasChanges() {
				s.confirmClose = true
				return s, nil, SettingsNone
			}
			return s, nil, SettingsClose

		case key.Matches(msg, keys.DefaultKeyMap.Enter):
			if s.cursor == settingsFieldTheme {
				s.themePicker = components.NewThemePickerModel(s.selectedThemeSlug())
				s.themePicker.SetSize(s.width, s.height)
				s.showThemePicker = true
				return s, nil, SettingsNone
			}
			if s.cursor == settingsFieldIdentity {
				s.picker = components.NewIdentityPickerModel(s.theme, s.provider)
				s.picker.SetSize(s.width, s.height)
				s.showPicker = true
				return s, nil, SettingsNone
			}
			if s.cursor == settingsFieldManageIds {
				return s, nil, SettingsManageIdentities
			}
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
			if s.cursor == settingsFieldTimeFormat {
				s.timeFormatIndex--
				if s.timeFormatIndex < 0 {
					s.timeFormatIndex = len(timeFormats) - 1
				}
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
			if s.cursor == settingsFieldTimeFormat {
				s.timeFormatIndex++
				if s.timeFormatIndex >= len(timeFormats) {
					s.timeFormatIndex = 0
				}
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
	case settingsFieldHistory:
		s.historyInput, cmd = s.historyInput.Update(msg)
	}
	return s, cmd, SettingsNone
}

// save validates and persists the config to disk.
func (s SettingsView) save() (SettingsView, tea.Cmd, SettingsAction) {
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
	s.config.MaxHistory = maxHistory
	s.config.TimeFormat = s.selectedTimeFormat()

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
	if s.confirmClose {
		return s.viewConfirmClose()
	}
	if s.showThemePicker {
		return s.themePicker.View()
	}
	if s.showPicker {
		return s.picker.View()
	}

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

	timeFormatDisplay := fmt.Sprintf("< %s >  (%d/%d)", s.selectedTimeFormat(), s.timeFormatIndex+1, len(timeFormats))

	// Count identities for the manage row label
	idCount := 0
	if s.provider != nil {
		if sums, err := s.provider.List(); err == nil {
			idCount = len(sums)
		}
	}
	manageLabel := "Manage Identities..."
	if idCount > 0 {
		manageLabel = fmt.Sprintf("Manage Identities (%d)...", idCount)
	}

	rows := []settingsRow{
		{"Theme", themeDisplay, false, ""},
		{"Default Identity", "", true, s.identityInput.View()},
		{"Max History", "", true, s.historyInput.View()},
		{"Time Format", timeFormatDisplay, false, ""},
		{"Identities", manageLabel, false, ""},
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

	// Help line
	b.WriteString("\n")
	b.WriteString("  " + s.renderHelp() + "\n")

	return b.String()
}

// renderHelp renders the help line for the settings view.
func (s SettingsView) renderHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(s.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(s.theme.Base0D).Bold(true)

	hint := ""
	switch s.cursor {
	case settingsFieldTheme, settingsFieldTimeFormat:
		hint = fmt.Sprintf(
			"%s/%s cycle  %s browse  %s/%s navigate  %s cancel",
			keyStyle.Render("[left]"),
			keyStyle.Render("[right]"),
			keyStyle.Render("[enter]"),
			keyStyle.Render("[up]"),
			keyStyle.Render("[down]"),
			keyStyle.Render("[esc]"),
		)
	case settingsFieldManageIds:
		hint = fmt.Sprintf(
			"%s/%s navigate  %s open  %s cancel",
			keyStyle.Render("[up]"),
			keyStyle.Render("[down]"),
			keyStyle.Render("[enter]"),
			keyStyle.Render("[esc]"),
		)
	default:
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

// viewConfirmClose renders the save confirmation dialog.
func (s SettingsView) viewConfirmClose() string {
	bg := s.theme.Base00
	sty := styles.NewStyles(s.theme)
	textStyle := lipgloss.NewStyle().Foreground(s.theme.Base05)
	keyStyle := lipgloss.NewStyle().Foreground(s.theme.Base0D).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(s.theme.Base04)

	content := lipgloss.JoinVertical(lipgloss.Left,
		textStyle.Render("You have unsaved changes."),
		textStyle.Render("Save before leaving?"),
		"",
		dimStyle.Render(keyStyle.Render("[y]")+" save    "+keyStyle.Render("[n]")+" discard    "+keyStyle.Render("[esc]")+" cancel"),
	)

	modal := sty.ModalBorder.Width(44).Render(content)
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(bg))
}

