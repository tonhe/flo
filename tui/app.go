package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/internal/version"
	"github.com/tonhe/flo/tui/components"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
	"github.com/tonhe/flo/tui/views"
)

// AppState represents the current screen/view of the application.
type AppState int

const (
	StateDashboard AppState = iota
	StateSwitcher
	StateDetail
	StateIdentity
	StateBuilder
	StateSettings
	StateEditor
)

// TickMsg triggers a periodic UI refresh to pick up new poll data.
type TickMsg struct{}

// AppModel is the root Bubble Tea model that manages all views and state.
type AppModel struct {
	state      AppState
	theme      styles.Theme
	config     *config.Config
	manager    *engine.Manager
	provider   identity.Provider
	dashboard  views.DashboardView
	switcher   views.SwitcherView
	detail     views.DetailView
	identity   views.IdentityView
	builder    views.BuilderView
	editor     views.EditorView
	settings   views.SettingsView
	width      int
	height     int
	activeDash    string
	startDashName string // auto-start dashboard from --dashboard flag
	storePath     string
	confirmQuit        bool
	identityFromSettings bool
}

// NewAppModel creates a new AppModel with the given config, engine manager,
// and identity provider. If startDash is non-empty, that dashboard will be
// loaded and started automatically on Init.
func NewAppModel(cfg *config.Config, mgr *engine.Manager, provider identity.Provider, startDash string, storePath string) AppModel {
	theme := styles.DefaultTheme
	if t := styles.GetThemeByName(cfg.Theme); t != nil {
		theme = *t
	}
	dashView := views.NewDashboardView(theme)
	dashView.SetTimeFormat(cfg.TimeFormat)
	detailView := views.NewDetailView(theme)
	detailView.SetTimeFormat(cfg.TimeFormat)
	return AppModel{
		state:         StateDashboard,
		theme:         theme,
		config:        cfg,
		manager:       mgr,
		provider:      provider,
		dashboard:     dashView,
		switcher:      views.NewSwitcherView(theme),
		detail:        detailView,
		identity:      views.NewIdentityView(theme, provider),
		startDashName: startDash,
		storePath:     storePath,
	}
}

// autoStartMsg is sent after Init to trigger dashboard auto-loading.
type autoStartMsg struct {
	name string
}

// Init returns the initial command to start the tick loop and optionally
// auto-start a dashboard.
func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if m.startDashName != "" {
		name := m.startDashName
		cmds = append(cmds, func() tea.Msg {
			return autoStartMsg{name: name}
		})
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// Update handles messages and dispatches to the active view.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autoStartMsg:
		dashDir, err := config.GetDashboardsDir()
		if err != nil {
			return m, nil
		}
		path := filepath.Join(dashDir, msg.name+".toml")
		dash, err := dashboard.LoadDashboard(path)
		if err != nil {
			return m, nil
		}
		if m.provider != nil {
			_ = m.manager.Start(dash, m.provider)
		}
		m.activeDash = dash.Name
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body height = total - 1 (header) - 2 (status bar lines)
		bodyHeight := msg.Height - 3
		m.dashboard.SetSize(msg.Width, bodyHeight)
		m.detail.SetSize(msg.Width, bodyHeight)
		m.identity.SetSize(msg.Width, bodyHeight)
		m.builder.SetSize(msg.Width, bodyHeight)
		m.editor.SetSize(msg.Width, bodyHeight)
		m.settings.SetSize(msg.Width, bodyHeight)
		return m, nil

	case TickMsg:
		// Refresh snapshot without blocking — if a poll cycle holds the
		// write lock, we use the most recent cached snapshot instead of
		// freezing the entire Bubble Tea event loop.
		if m.activeDash != "" {
			if snap := m.manager.TryGetSnapshot(m.activeDash); snap != nil {
				m.dashboard.SetSnapshot(snap)
				if m.state == StateDetail {
					label, iface := m.dashboard.SelectedInterface()
					if iface != nil {
						m.detail.SetInterface(label, iface)
					}
				}
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		// Confirm-to-quit dialog intercepts all keys when visible
		if m.confirmQuit {
			switch msg.String() {
			case "y":
				m.manager.StopAll()
				return m, tea.Quit
			case "n", "esc":
				m.confirmQuit = false
				return m, nil
			}
			return m, nil
		}

		// Global key bindings
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Quit):
			return m.tryQuit()
		}

		// State-specific key handling
		switch m.state {
		case StateDashboard:
			// 'q' to quit (only in dashboard, not in text-input views)
			if msg.String() == "q" {
				return m.tryQuit()
			}
			// Open the dashboard switcher on 'd'
			if key.Matches(msg, keys.DefaultKeyMap.Dashboard) {
				m.state = StateSwitcher
				m.refreshSwitcher()
				return m, nil
			}
			// Open settings on 's'
			if key.Matches(msg, keys.DefaultKeyMap.Settings) {
				m.settings = views.NewSettingsView(m.theme, m.config, m.provider)
				m.settings.SetSize(m.width, m.height-3)
				m.state = StateSettings
				return m, nil
			}
			// The following keys only work when a dashboard is loaded
			if m.activeDash != "" {
				// Open the dashboard editor on 'e'
				if key.Matches(msg, keys.DefaultKeyMap.Edit) {
					m.editActiveDashboard()
					return m, nil
				}
				// Force refresh on 'r'
				if key.Matches(msg, keys.DefaultKeyMap.Refresh) {
					if snap := m.manager.TryGetSnapshot(m.activeDash); snap != nil {
						m.dashboard.SetSnapshot(snap)
					}
					return m, nil
				}
				// Open detail view on Enter
				if key.Matches(msg, keys.DefaultKeyMap.Enter) {
					label, iface := m.dashboard.SelectedInterface()
					if iface != nil {
						m.detail.SetInterface(label, iface)
						m.state = StateDetail
					}
					return m, nil
				}
			}
			var cmd tea.Cmd
			m.dashboard, cmd = m.dashboard.Update(msg)
			return m, cmd

		case StateDetail:
			if msg.String() == "q" {
				return m.tryQuit()
			}
			var cmd tea.Cmd
			var goBack bool
			m.detail, cmd, goBack = m.detail.Update(msg)
			if goBack {
				m.state = StateDashboard
				return m, nil
			}
			return m, cmd

		case StateIdentity:
			var cmd tea.Cmd
			var goBack bool
			m.identity, cmd, goBack = m.identity.Update(msg)
			// Propagate newly created provider from identity view to app model
			if m.provider == nil {
				if p := m.identity.Provider(); p != nil {
					m.provider = p
				}
			}
			if goBack {
				if m.identityFromSettings {
					m.identityFromSettings = false
					m.settings = views.NewSettingsView(m.theme, m.config, m.provider)
					m.settings.SetSize(m.width, m.height-3)
					m.state = StateSettings
				} else {
					m.state = StateDashboard
				}
				return m, nil
			}
			return m, cmd

		case StateSwitcher:
			if msg.String() == "q" {
				return m.tryQuit()
			}
			var cmd tea.Cmd
			var action views.SwitcherAction
			m.switcher, cmd, action = m.switcher.Update(msg)

			switch action {
			case views.ActionClose:
				m.state = StateDashboard
				return m, nil

			case views.ActionSwitch:
				if item := m.switcher.SelectedItem(); item != nil {
					m.switchToDashboard(item)
				}
				m.state = StateDashboard
				return m, nil

			case views.ActionStop:
				if item := m.switcher.SelectedItem(); item != nil && item.Running {
					_ = m.manager.Stop(item.Name)
					if m.activeDash == item.Name {
						m.activeDash = ""
						m.dashboard.SetSnapshot(nil)
					}
					m.refreshSwitcher()
				}
				return m, nil

			case views.ActionNew:
				m.builder = views.NewBuilderView(m.theme, m.provider)
				m.builder.SetSize(m.width, m.height-3)
				m.state = StateBuilder
				return m, nil

			case views.ActionEdit:
				if item := m.switcher.SelectedItem(); item != nil {
					m.editDashboard(item.FilePath)
				}
				return m, nil
			}
			return m, cmd

		case StateBuilder:
			var cmd tea.Cmd
			var action views.BuilderAction
			m.builder, cmd, action = m.builder.Update(msg)

			switch action {
			case views.BuilderActionClose:
				m.state = StateDashboard
				return m, nil

			case views.BuilderActionSave:
				m.applyDashboardSave(m.builder.SavedPath)
				m.state = StateDashboard
				return m, nil
			}
			return m, cmd

		case StateSettings:
			var cmd tea.Cmd
			var action views.SettingsAction
			m.settings, cmd, action = m.settings.Update(msg)

			switch action {
			case views.SettingsClose:
				m.state = StateDashboard
				return m, nil

			case views.SettingsSaved:
				// Apply the new theme to all views
				if t := styles.GetThemeByName(m.settings.SavedTheme); t != nil {
					m.theme = *t
					bodyHeight := m.height - 3
					m.dashboard = views.NewDashboardView(m.theme)
					m.dashboard.SetSize(m.width, bodyHeight)
					m.dashboard.SetTimeFormat(m.config.TimeFormat)
					m.switcher = views.NewSwitcherView(m.theme)
					m.switcher.SetSize(m.width, bodyHeight)
					m.detail = views.NewDetailView(m.theme)
					m.detail.SetSize(m.width, bodyHeight)
					m.detail.SetTimeFormat(m.config.TimeFormat)
					m.identity = views.NewIdentityView(m.theme, m.provider)
					m.identity.SetSize(m.width, bodyHeight)
				}
				m.state = StateDashboard
				return m, nil

			case views.SettingsManageIdentities:
				m.identity = views.NewIdentityView(m.theme, m.provider)
				m.identity.SetSize(m.width, m.height-3)
				m.identity.SetStorePath(m.storePath)
				m.identity.SetOnStoreCreated(func(p identity.Provider) {
					m.provider = p
				})
				m.identity.Refresh()
				m.identityFromSettings = true
				m.state = StateIdentity
				return m, nil
			}
			return m, cmd

		case StateEditor:
			var cmd tea.Cmd
			var action views.EditorAction
			m.editor, cmd, action = m.editor.Update(msg)
			switch action {
			case views.EditorActionSaved:
				m.applyDashboardSave(m.editor.SavedPath)
				m.state = StateDashboard
				return m, nil
			case views.EditorActionClose:
				m.state = StateDashboard
				return m, nil
			}
			return m, cmd
		}

	default:
		// Forward non-key messages (spinner ticks, async results, etc.)
		// to views that host async sub-components.
		switch m.state {
		case StateBuilder:
			var cmd tea.Cmd
			var action views.BuilderAction
			m.builder, cmd, action = m.builder.Update(msg)
			if action == views.BuilderActionSave {
				m.applyDashboardSave(m.builder.SavedPath)
				m.state = StateDashboard
			} else if action == views.BuilderActionClose {
				m.state = StateDashboard
			}
			return m, cmd

		case StateEditor:
			var cmd tea.Cmd
			var action views.EditorAction
			m.editor, cmd, action = m.editor.Update(msg)
			if action == views.EditorActionSaved {
				m.applyDashboardSave(m.editor.SavedPath)
				m.state = StateDashboard
			} else if action == views.EditorActionClose {
				m.state = StateDashboard
			}
			return m, cmd
		}
	}
	return m, nil
}

// refreshSwitcher reloads the dashboard list into the switcher view.
func (m *AppModel) refreshSwitcher() {
	dashDir, err := config.GetDashboardsDir()
	if err != nil {
		return
	}
	m.switcher.SetSize(m.width, m.height-3)
	m.switcher.Refresh(dashDir, m.manager)
}

// switchToDashboard loads and activates the selected dashboard, starting its
// engine if it is not already running.
func (m *AppModel) switchToDashboard(item *views.SwitcherItem) {
	// If this engine is already running, just switch the active view.
	if item.Running {
		m.activeDash = item.Name
		return
	}

	// Load the dashboard TOML and start a new engine.
	dash, err := dashboard.LoadDashboard(item.FilePath)
	if err != nil {
		return
	}

	if m.provider != nil {
		if err := m.manager.Start(dash, m.provider); err != nil {
			return
		}
	}
	m.activeDash = dash.Name
}

// applyDashboardSave loads a saved dashboard and restarts its engine only if
// polling-relevant fields changed. Cosmetic edits (labels) preserve graph data.
func (m *AppModel) applyDashboardSave(path string) {
	newDash, err := dashboard.LoadDashboard(path)
	if err != nil {
		return
	}

	// Check if the running engine needs a restart
	if m.activeDash != "" {
		oldDash := m.manager.GetDashboard(m.activeDash)
		if oldDash != nil && !dashboard.NeedsRestart(oldDash, newDash) {
			// No restart needed — keep engine running with existing data
			m.activeDash = newDash.Name
			return
		}
		_ = m.manager.Stop(m.activeDash)
	}

	if m.provider != nil {
		_ = m.manager.Start(newDash, m.provider)
	}
	m.activeDash = newDash.Name
}

// editDashboard loads a dashboard TOML and opens it in the editor for editing.
func (m *AppModel) editDashboard(path string) {
	dash, err := dashboard.LoadDashboard(path)
	if err != nil {
		return
	}
	m.editor = views.NewEditorView(m.theme, m.provider)
	m.editor.LoadDashboard(dash, path)
	m.editor.SetSize(m.width, m.height-3)
	m.state = StateEditor
}

// editActiveDashboard opens the currently active dashboard for editing.
func (m *AppModel) editActiveDashboard() {
	dashDir, err := config.GetDashboardsDir()
	if err != nil {
		return
	}
	names, err := dashboard.ListDashboards(dashDir)
	if err != nil {
		return
	}
	for _, name := range names {
		path := filepath.Join(dashDir, name+".toml")
		dash, loadErr := dashboard.LoadDashboard(path)
		if loadErr == nil && dash.Name == m.activeDash {
			m.editDashboard(path)
			return
		}
	}
}

// tryQuit either quits immediately (no engines running) or shows a confirmation dialog.
func (m AppModel) tryQuit() (tea.Model, tea.Cmd) {
	if len(m.manager.TryListEngines()) == 0 {
		return m, tea.Quit
	}
	m.confirmQuit = true
	return m, nil
}

// renderQuitConfirm renders the quit confirmation modal dialog.
func (m AppModel) renderQuitConfirm(theme styles.Theme) string {
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	sty := styles.NewStyles(theme)
	textStyle := lipgloss.NewStyle().Foreground(theme.Base05)
	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04)

	content := lipgloss.JoinVertical(lipgloss.Left,
		textStyle.Render("Engines are still running."),
		textStyle.Render("Quit anyway?"),
		"",
		dimStyle.Render(keyStyle.Render("[y]")+" yes    "+keyStyle.Render("[n]")+" no"),
	)

	modal := sty.ModalBorder.Width(36).Render(content)
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(theme.Base00))
}

// View renders the full application UI by composing header, body, and status.
func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Use the settings preview theme for immersive rendering when in settings.
	renderTheme := m.theme
	if m.state == StateSettings {
		renderTheme = m.settings.PreviewTheme()
	}

	// Header bar (non-blocking to avoid freezing during poll cycles)
	engineList := m.manager.TryListEngines()
	header := components.RenderHeader(
		renderTheme,
		m.activeDash,
		m.activeDash != "",
		len(engineList),
		len(engineList),
		m.width,
		version.Version,
		version.Build,
	)

	// Body content based on current state
	var body string
	switch m.state {
	case StateDashboard:
		body = m.dashboard.View()
	case StateSwitcher:
		// Render the dashboard view underneath, then overlay the switcher modal
		body = m.dashboard.View()
	case StateDetail:
		body = m.detail.View()
	case StateIdentity:
		body = m.identity.View()
	case StateBuilder:
		body = m.builder.View()
	case StateSettings:
		body = m.settings.View()
	case StateEditor:
		body = m.editor.View()
	default:
		body = "View not implemented"
	}

	// Gather status bar metrics (non-blocking)
	var lastPoll time.Time
	var interval time.Duration
	okCount, totalCount := 0, 0
	if m.activeDash != "" {
		if snap := m.manager.TryGetSnapshot(m.activeDash); snap != nil {
			lastPoll = snap.LastPoll
			interval = snap.Interval
			for _, g := range snap.Groups {
				for _, t := range g.Targets {
					for _, iface := range t.Interfaces {
						totalCount++
						if iface.PollError == nil {
							okCount++
						}
					}
				}
			}
		}
	}

	// Per-state key hints for the status bar
	var hints []components.KeyHint
	switch m.state {
	case StateDashboard:
		hints = []components.KeyHint{
			{"d", "dashboards"}, {"s", "settings"},
		}
		if m.activeDash != "" {
			hints = append(hints,
				components.KeyHint{Key: "enter", Desc: "detail"},
				components.KeyHint{Key: "e", Desc: "edit"},
				components.KeyHint{Key: "r", Desc: "refresh"},
			)
		}
		hints = append(hints, components.KeyHint{Key: "q", Desc: "quit"})
	case StateDetail:
		hints = []components.KeyHint{
			{"esc", "back"}, {"q", "quit"},
		}
	case StateSwitcher:
		hints = []components.KeyHint{
			{"enter", "switch"}, {"n", "new"}, {"e", "edit"},
			{"x", "stop"}, {"esc", "close"}, {"q", "quit"},
		}
	case StateIdentity:
		hints = []components.KeyHint{
			{"esc", "back"}, {"ctrl+c", "quit"},
		}
	case StateBuilder:
		hints = []components.KeyHint{
			{"esc", "cancel"}, {"ctrl+c", "quit"},
		}
	case StateEditor:
		hints = []components.KeyHint{
			{"esc", "save & close"}, {"ctrl+c", "quit"},
		}
	case StateSettings:
		hints = []components.KeyHint{
			{"esc", "back"}, {"ctrl+c", "quit"},
		}
	}

	statusBar := components.RenderStatusBar(renderTheme, interval, lastPoll, okCount, totalCount, m.width, hints)

	// Fill body to the available height between header and status bar
	bodyHeight := m.height - 1 - 2 // 1 header line, 2 status bar lines
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	// When a modal is active, use its already-centered output as the body
	// instead of overlaying on top of the full screen. Both switcher and help
	// call lipgloss.Place() internally, so their output is already sized to
	// width x bodyHeight.
	if m.confirmQuit {
		body = m.renderQuitConfirm(renderTheme)
	} else if m.state == StateSwitcher {
		body = m.switcher.View()
	}

	filledBody := fillBackground(body, m.width, bodyHeight, renderTheme.Base00)
	full := lipgloss.JoinVertical(lipgloss.Left, header, filledBody, statusBar)
	return full
}

// fillBackground post-processes rendered ANSI output to ensure the theme
// background covers every character cell. lipgloss child styles emit
// \x1b[0m resets that kill the outer background, leaving the terminal's
// default black showing through. This function operates at the raw ANSI
// level: it injects the background escape code after every reset sequence
// so the theme background is always active.
func fillBackground(content string, width, height int, bg lipgloss.Color) string {
	bgCode := hexToANSIBg(string(bg))
	reset := "\x1b[0m"

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Re-inject background after every reset within the line
		patched := strings.ReplaceAll(line, reset, reset+bgCode)
		// Prepend background at start of line
		patched = bgCode + patched
		// Pad to full width with spaces (bg is already active)
		visible := lipgloss.Width(patched)
		if visible < width {
			patched += strings.Repeat(" ", width-visible)
		}
		// Close with a reset so the next line starts clean
		lines[i] = patched + reset
	}
	// Fill remaining height with empty background rows
	emptyRow := bgCode + strings.Repeat(" ", width) + reset
	for len(lines) < height {
		lines = append(lines, emptyRow)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// hexToANSIBg converts a hex color string (e.g. "#1e1e2e") to a raw
// 24-bit ANSI background escape sequence.
func hexToANSIBg(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

