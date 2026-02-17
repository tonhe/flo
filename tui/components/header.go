package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

// RenderHeader renders the top header bar with app name, dashboard name,
// live/stopped status, and engine count.
func RenderHeader(theme styles.Theme, dashName string, isLive bool, activeCount, totalCount, width int) string {
	left := lipgloss.NewStyle().
		Foreground(theme.Base0D).
		Bold(true).
		Render("flo")

	displayName := dashName
	if displayName == "" {
		displayName = "(no dashboard)"
	}
	center := lipgloss.NewStyle().
		Foreground(theme.Base05).
		Render(displayName)

	status := "STOPPED"
	statusColor := theme.Base08
	if isLive {
		status = "LIVE"
		statusColor = theme.Base0B
	}
	right := lipgloss.NewStyle().
		Foreground(statusColor).
		Render(status)

	engines := lipgloss.NewStyle().
		Foreground(theme.Base04).
		Render(fmt.Sprintf("%d/%d engines", activeCount, totalCount))

	content := fmt.Sprintf(" %s  |  %s  |  %s  |  %s ", left, center, right, engines)

	return lipgloss.NewStyle().
		Background(theme.Base01).
		Width(width).
		Render(content)
}
