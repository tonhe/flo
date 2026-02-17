package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/engine"
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
	dashboard  views.DashboardView
	width      int
	height     int
	activeDash string
}

// NewAppModel creates a new AppModel with the given config and engine manager.
func NewAppModel(cfg *config.Config, mgr *engine.Manager) AppModel {
	theme := styles.DefaultTheme
	if t := styles.GetThemeByName(cfg.Theme); t != nil {
		theme = *t
	}
	return AppModel{
		state:     StateDashboard,
		theme:     theme,
		config:    cfg,
		manager:   mgr,
		dashboard: views.NewDashboardView(theme),
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
		m.dashboard.SetSize(msg.Width, msg.Height-3)
		return m, nil

	case TickMsg:
		// Refresh snapshot from the engine manager
		if m.activeDash != "" {
			if snap, err := m.manager.GetSnapshot(m.activeDash); err == nil {
				m.dashboard.SetSnapshot(snap)
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
			var cmd tea.Cmd
			m.dashboard, cmd = m.dashboard.Update(msg)
			return m, cmd
		}
	}
	return m, nil
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

	return lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(body), statusBar)
}
