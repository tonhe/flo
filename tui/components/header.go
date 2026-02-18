package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

// RenderHeader renders the top header bar with app name, dashboard name,
// live/stopped status, and engine count.
func RenderHeader(theme styles.Theme, dashName string, isLive bool, activeCount, totalCount, width int, ver, build string) string {
	bg := theme.Base01
	sep := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg).Render("  |  ")
	pad := lipgloss.NewStyle().Background(bg).Render(" ")

	left := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true).Render("flo")

	displayName := dashName
	if displayName == "" {
		displayName = "(no dashboard)"
	}
	center := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg).Render(displayName)

	status := "STOPPED"
	statusColor := theme.Base08
	if isLive {
		status = "LIVE"
		statusColor = theme.Base0B
	}
	right := lipgloss.NewStyle().Foreground(statusColor).Background(bg).Render(status)

	engines := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg).
		Render(fmt.Sprintf("%d/%d engines", activeCount, totalCount))

	versionStr := "v" + ver
	if build != "" {
		versionStr += "  " + build
	}
	versionSeg := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg).Render(versionStr)

	content := pad + left + sep + center + sep + right + sep + engines + sep + versionSeg

	contentWidth := lipgloss.Width(content)
	if contentWidth < width {
		filler := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-contentWidth))
		content += filler
	}
	return content
}
