package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/internal/identity"
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
	width      int
	height     int
	activeDash string
}

// NewAppModel creates a new AppModel with the given config, engine manager,
// and identity provider.
func NewAppModel(cfg *config.Config, mgr *engine.Manager, provider identity.Provider) AppModel {
	theme := styles.DefaultTheme
	if t := styles.GetThemeByName(cfg.Theme); t != nil {
		theme = *t
	}
	return AppModel{
		state:     StateDashboard,
		theme:     theme,
		config:    cfg,
		manager:   mgr,
		provider:  provider,
		dashboard: views.NewDashboardView(theme),
		switcher:  views.NewSwitcherView(theme),
		detail:    views.NewDetailView(theme),
		identity:  views.NewIdentityView(theme, provider),
	}
}

// Init returns the initial command to start the tick loop.
func (m AppModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// Update handles messages and dispatches to the active view.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body height = total - 1 (header) - 2 (status bar lines)
		bodyHeight := msg.Height - 3
		m.dashboard.SetSize(msg.Width, bodyHeight)
		m.detail.SetSize(msg.Width, bodyHeight)
		m.identity.SetSize(msg.Width, bodyHeight)
		m.builder.SetSize(msg.Width, bodyHeight)
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
		// Global key bindings
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Quit):
			m.manager.StopAll()
			return m, tea.Quit
		}

		// State-specific key handling
		switch m.state {
		case StateDashboard:
			// Open the dashboard switcher on 'd'
			if key.Matches(msg, keys.DefaultKeyMap.Dashboard) {
				m.state = StateSwitcher
				m.refreshSwitcher()
				return m, nil
			}
			// Open the identity manager on 'i'
			if key.Matches(msg, keys.DefaultKeyMap.Identity) {
				m.identity.SetSize(m.width, m.height-3)
				m.identity.Refresh()
				m.state = StateIdentity
				return m, nil
			}
			// Open the dashboard builder on 'n'
			if key.Matches(msg, keys.DefaultKeyMap.New) {
				m.builder = views.NewBuilderView(m.theme, m.provider)
				m.builder.SetSize(m.width, m.height-3)
				m.state = StateBuilder
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
				// Load the saved dashboard and start its engine
				path := m.builder.SavedPath
				if dash, err := dashboard.LoadDashboard(path); err == nil {
					if m.provider != nil {
						_ = m.manager.Start(dash, m.provider)
					}
					m.activeDash = dash.Name
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

	full := lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(body), statusBar)

	// When the switcher is active, overlay it on top of the composed screen.
	if m.state == StateSwitcher {
		overlay := m.switcher.View()
		full = overlayCenter(full, overlay, m.width, m.height)
	}

	return full
}

// overlayCenter composites the overlay string on top of the background string,
// centering the overlay within the given width and height. Non-space characters
// in the overlay replace the corresponding characters in the background.
func overlayCenter(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	// Pad background to full height if needed
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	// Calculate vertical offset to center the overlay
	yOff := (height - len(fgLines)) / 2
	if yOff < 0 {
		yOff = 0
	}

	for i, fgLine := range fgLines {
		bgIdx := yOff + i
		if bgIdx >= len(bgLines) {
			break
		}

		bgRunes := []rune(bgLines[bgIdx])
		fgRunes := []rune(fgLine)

		// Calculate horizontal offset to center this line
		xOff := (width - len(fgRunes)) / 2
		if xOff < 0 {
			xOff = 0
		}

		// Ensure the background line is wide enough
		for len(bgRunes) < width {
			bgRunes = append(bgRunes, ' ')
		}

		// Overlay foreground onto background
		for j, r := range fgRunes {
			pos := xOff + j
			if pos < len(bgRunes) {
				bgRunes[pos] = r
			}
		}
		bgLines[bgIdx] = string(bgRunes)
	}

	return strings.Join(bgLines, "\n")
}
