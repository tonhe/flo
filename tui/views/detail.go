package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/tui/components"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// DetailView is a split-screen view showing interface information at the top
// and In/Out traffic charts at the bottom.
type DetailView struct {
	theme       styles.Theme
	sty         *styles.Styles
	targetLabel string
	ifaceStats  *engine.InterfaceStats
	width       int
	height      int
}

// NewDetailView creates a new DetailView with the given theme.
func NewDetailView(theme styles.Theme) DetailView {
	return DetailView{
		theme: theme,
		sty:   styles.NewStyles(theme),
	}
}

// SetInterface updates the detail view with new interface data.
func (v *DetailView) SetInterface(label string, stats *engine.InterfaceStats) {
	v.targetLabel = label
	v.ifaceStats = stats
}

// SetSize updates the available dimensions for the view.
func (v *DetailView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Update handles key messages for the detail view. The third return value
// indicates whether the user wants to go back (Esc pressed).
func (v DetailView) Update(msg tea.Msg) (DetailView, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return v, nil, true
		}
	}
	return v, nil, false
}

// View renders the detail view with an info panel and traffic charts.
func (v DetailView) View() string {
	if v.ifaceStats == nil {
		return v.renderEmpty()
	}
	return v.renderDetail()
}

// renderEmpty shows a placeholder when no interface is selected.
func (v DetailView) renderEmpty() string {
	msg := lipgloss.NewStyle().
		Foreground(v.theme.Base04).
		Align(lipgloss.Center).
		Render("No interface selected")
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, msg)
}

// renderDetail renders the full split-screen detail view.
func (v DetailView) renderDetail() string {
	iface := v.ifaceStats

	// --- Top section: interface info panel ---
	infoPanel := v.renderInfoPanel(iface)

	// --- Bottom section: two charts side by side ---
	// Calculate chart dimensions
	infoPanelHeight := 10 // fixed info panel height
	chartHeight := v.height - infoPanelHeight
	if chartHeight < 6 {
		chartHeight = 6
	}
	chartWidth := (v.width - 3) / 2 // 3 chars for separator and padding
	if chartWidth < 15 {
		chartWidth = 15
	}

	// Extract rate data from history
	inData, outData := v.extractRateData()

	// Render charts
	inChart := components.RenderChart(inData, chartWidth, chartHeight, "In Traffic")
	outChart := components.RenderChart(outData, chartWidth, chartHeight, "Out Traffic")

	// Color the chart output
	inChartStyled := lipgloss.NewStyle().
		Foreground(v.theme.Base0B). // green for in
		Render(inChart)
	outChartStyled := lipgloss.NewStyle().
		Foreground(v.theme.Base0C). // cyan for out
		Render(outChart)

	// Join charts side by side with a separator
	sep := lipgloss.NewStyle().
		Foreground(v.theme.Base03).
		Render(strings.Repeat(" | \n", chartHeight))
	// Use lipgloss to join horizontally
	chartsSection := lipgloss.JoinHorizontal(lipgloss.Top, inChartStyled, sep, outChartStyled)

	// Compose final layout: info panel on top, charts on bottom
	helpLine := v.renderHelp()
	full := lipgloss.JoinVertical(lipgloss.Left, infoPanel, "", chartsSection, helpLine)

	return full
}

// renderInfoPanel renders the interface information section at the top.
func (v DetailView) renderInfoPanel(iface *engine.InterfaceStats) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base04).
		Width(16)
	valueStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base05)
	highlightStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)

	// Status color
	statusStyle := lipgloss.NewStyle().Foreground(v.theme.Base0A)
	switch iface.Status {
	case "up":
		statusStyle = lipgloss.NewStyle().Foreground(v.theme.Base0B)
	case "down":
		statusStyle = lipgloss.NewStyle().Foreground(v.theme.Base08)
	}

	// Speed formatting
	speedStr := formatSpeed(iface.Speed)

	// Utilization with threshold coloring
	utilStyle := lipgloss.NewStyle().Foreground(v.theme.Base0B)
	switch {
	case iface.Utilization >= 80:
		utilStyle = lipgloss.NewStyle().Foreground(v.theme.Base08)
	case iface.Utilization >= 50:
		utilStyle = lipgloss.NewStyle().Foreground(v.theme.Base0A)
	}

	// Build info rows
	rows := []string{
		"",
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Device:"),
			highlightStyle.Render(v.targetLabel)),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Interface:"),
			highlightStyle.Render(iface.Name)),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Description:"),
			valueStyle.Render(iface.Description)),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Status:"),
			statusStyle.Render(iface.Status)),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Speed:"),
			valueStyle.Render(speedStr)),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Current In:"),
			valueStyle.Render(components.FormatRate(iface.InRate))),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Current Out:"),
			valueStyle.Render(components.FormatRate(iface.OutRate))),
		fmt.Sprintf("  %s%s",
			labelStyle.Render("Utilization:"),
			utilStyle.Render(fmt.Sprintf("%.1f%%", iface.Utilization))),
	}

	return strings.Join(rows, "\n")
}

// renderHelp renders a help line at the bottom of the detail view.
func (v DetailView) renderHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
	return helpStyle.Render(fmt.Sprintf("  %s to go back", keyStyle.Render("[esc]")))
}

// extractRateData pulls InRate and OutRate slices from the interface history.
func (v DetailView) extractRateData() (inData, outData []float64) {
	if v.ifaceStats == nil || v.ifaceStats.History == nil {
		return nil, nil
	}

	samples := v.ifaceStats.History.All()
	if len(samples) == 0 {
		return nil, nil
	}

	inData = make([]float64, len(samples))
	outData = make([]float64, len(samples))
	for i, s := range samples {
		inData[i] = s.InRate
		outData[i] = s.OutRate
	}
	return inData, outData
}

// formatSpeed converts an interface speed in Mbps to a human-readable string.
func formatSpeed(speedMbps uint64) string {
	switch {
	case speedMbps == 0:
		return "unknown"
	case speedMbps >= 1000000:
		return fmt.Sprintf("%.0fT", float64(speedMbps)/1000000)
	case speedMbps >= 1000:
		return fmt.Sprintf("%.0fG", float64(speedMbps)/1000)
	default:
		return fmt.Sprintf("%dM", speedMbps)
	}
}
