package tui

import (
	"path/filepath"
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
	settings   views.SettingsView
	help       views.HelpView
	width      int
	height     int
	activeDash    string
	startDashName string // auto-start dashboard from --dashboard flag
	storePath     string
	confirmQuit   bool
}

// NewAppModel creates a new AppModel with the given config, engine manager,
// and identity provider. If startDash is non-empty, that dashboard will be
// loaded and started automatically on Init.
func NewAppModel(cfg *config.Config, mgr *engine.Manager, provider identity.Provider, startDash string, storePath string) AppModel {
	theme := styles.DefaultTheme
	if t := styles.GetThemeByName(cfg.Theme); t != nil {
		theme = *t
	}
	return AppModel{
		state:         StateDashboard,
		theme:         theme,
		config:        cfg,
		manager:       mgr,
		provider:      provider,
		dashboard:     views.NewDashboardView(theme),
		switcher:      views.NewSwitcherView(theme),
		detail:        views.NewDetailView(theme),
		identity:      views.NewIdentityView(theme, provider),
		help:          views.NewHelpView(theme),
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
		m.settings.SetSize(msg.Width, bodyHeight)
		m.help.SetSize(msg.Width, bodyHeight)
		return m, nil

	case TickMsg:
		// Refresh snapshot from the engine manager
		if m.activeDash != "" {
			if snap, err := m.manager.GetSnapshot(m.activeDash); err == nil {
				m.dashboard.SetSnapshot(snap)
				// If viewing detail, refresh the selected interface data
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
		// Help overlay intercepts all keys when visible
		if m.help.IsVisible() {
			if key.Matches(msg, keys.DefaultKeyMap.Help) || key.Matches(msg, keys.DefaultKeyMap.Escape) {
				m.help.Toggle()
			}
			return m, nil
		}

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
		case key.Matches(msg, keys.DefaultKeyMap.Help):
			bodyHeight := m.height - 3
			if bodyHeight < 1 {
				bodyHeight = 1
			}
			m.help.SetSize(m.width, bodyHeight)
			m.help.Toggle()
			return m, nil
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
			// Open the identity manager on 'i'
			if key.Matches(msg, keys.DefaultKeyMap.Identity) {
				m.identity.SetSize(m.width, m.height-3)
				m.identity.SetStorePath(m.storePath)
				m.identity.SetOnStoreCreated(func(p identity.Provider) {
					m.provider = p
				})
				m.identity.Refresh()
				m.state = StateIdentity
				return m, nil
			}
			// Open the dashboard editor on 'e'
			if key.Matches(msg, keys.DefaultKeyMap.Edit) {
				if m.activeDash != "" {
					m.editActiveDashboard()
				}
				return m, nil
			}
			// Open settings on 's'
			if key.Matches(msg, keys.DefaultKeyMap.Settings) {
				m.settings = views.NewSettingsView(m.theme, m.config)
				m.settings.SetSize(m.width, m.height-3)
				m.state = StateSettings
				return m, nil
			}
			// Force refresh on 'r'
			if key.Matches(msg, keys.DefaultKeyMap.Refresh) {
				if m.activeDash != "" {
					if snap, err := m.manager.GetSnapshot(m.activeDash); err == nil {
						m.dashboard.SetSnapshot(snap)
					}
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
			if goBack {
				m.state = StateDashboard
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
				path := m.builder.SavedPath
				if dash, err := dashboard.LoadDashboard(path); err == nil {
					// Stop old engine if editing an active dashboard
					if m.activeDash != "" {
						_ = m.manager.Stop(m.activeDash)
					}
					if m.provider != nil {
						_ = m.manager.Start(dash, m.provider)
					}
					m.activeDash = dash.Name
				}
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
					m.dashboard = views.NewDashboardView(m.theme)
					m.dashboard.SetSize(m.width, m.height-3)
					m.switcher = views.NewSwitcherView(m.theme)
					m.detail = views.NewDetailView(m.theme)
					m.identity = views.NewIdentityView(m.theme, m.provider)
					m.help = views.NewHelpView(m.theme)
				}
				m.state = StateDashboard
				return m, nil
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

// editDashboard loads a dashboard TOML and opens it in the builder for editing.
func (m *AppModel) editDashboard(path string) {
	dash, err := dashboard.LoadDashboard(path)
	if err != nil {
		return
	}
	m.builder = views.NewBuilderView(m.theme, m.provider)
	m.builder.LoadDashboard(dash, path)
	m.builder.SetSize(m.width, m.height-3)
	m.state = StateBuilder
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
	if len(m.manager.ListEngines()) == 0 {
		return m, tea.Quit
	}
	m.confirmQuit = true
	return m, nil
}

// renderQuitConfirm renders the quit confirmation modal dialog.
func (m AppModel) renderQuitConfirm() string {
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	sty := styles.NewStyles(m.theme)
	textStyle := lipgloss.NewStyle().Foreground(m.theme.Base05)
	keyStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)

	content := lipgloss.JoinVertical(lipgloss.Left,
		textStyle.Render("Engines are still running."),
		textStyle.Render("Quit anyway?"),
		"",
		dimStyle.Render(keyStyle.Render("[y]")+" yes    "+keyStyle.Render("[n]")+" no"),
	)

	modal := sty.ModalBorder.Width(36).Render(content)
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, modal)
}

// View renders the full application UI by composing header, body, and status.
func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header bar
	engineList := m.manager.ListEngines()
	header := components.RenderHeader(
		m.theme,
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
	default:
		body = "View not implemented"
	}

	// Gather status bar metrics
	var lastPoll time.Time
	var interval time.Duration
	okCount, totalCount := 0, 0
	if m.activeDash != "" {
		if snap, err := m.manager.GetSnapshot(m.activeDash); err == nil {
			lastPoll = snap.LastPoll
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
		interval = m.config.PollInterval
	}

	statusBar := components.RenderStatusBar(m.theme, interval, lastPoll, okCount, totalCount, m.width)

	// Fill body to the available height between header and status bar
	bodyHeight := m.height - 1 - 2 // 1 header line, 2 status bar lines
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	bodyStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		Background(m.theme.Base00).
		Foreground(m.theme.Base05)

	// When a modal is active, use its already-centered output as the body
	// instead of overlaying on top of the full screen. Both switcher and help
	// call lipgloss.Place() internally, so their output is already sized to
	// width x bodyHeight.
	if m.confirmQuit {
		modalBody := m.renderQuitConfirm()
		full := lipgloss.JoinVertical(lipgloss.Left, header, modalBody, statusBar)
		return full
	}
	if m.help.IsVisible() {
		modalBody := m.help.View()
		full := lipgloss.JoinVertical(lipgloss.Left, header, modalBody, statusBar)
		return full
	}
	if m.state == StateSwitcher {
		modalBody := m.switcher.View()
		full := lipgloss.JoinVertical(lipgloss.Left, header, modalBody, statusBar)
		return full
	}

	full := lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(body), statusBar)
	return full
}

