package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

// RenderStatusBar renders the two-line status/footer bar showing poll info,
// health status, and key bindings.
func RenderStatusBar(theme styles.Theme, interval time.Duration, lastPoll time.Time, okCount, totalCount, width int) string {
	bg := theme.Base01
	bgStyle := lipgloss.NewStyle().Background(bg)
	sep := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg).Render(" | ")

	pollSeg := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg).Render(fmt.Sprintf("poll: %s", interval))
	lastStr := "never"
	if !lastPoll.IsZero() {
		lastStr = lastPoll.Format("15:04:05")
	}
	lastSeg := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg).Render(fmt.Sprintf("last: %s", lastStr))

	healthColor := theme.Base0B
	if okCount < totalCount {
		healthColor = theme.Base0A
	}
	healthSeg := lipgloss.NewStyle().Foreground(healthColor).Background(bg).
		Render(fmt.Sprintf("%d/%d OK", okCount, totalCount))

	topContent := bgStyle.Render(" ") + pollSeg + sep + lastSeg + sep + healthSeg
	topWidth := lipgloss.Width(topContent)
	if topWidth < width {
		topContent += bgStyle.Render(strings.Repeat(" ", width-topWidth))
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	spacer := bgStyle.Render("  ")

	keys := bgStyle.Render(" ") +
		keyStyle.Render("enter") + descStyle.Render(":detail") + spacer +
		keyStyle.Render("d") + descStyle.Render(":dashboards") + spacer +
		keyStyle.Render("i") + descStyle.Render(":identities") + spacer +
		keyStyle.Render("e") + descStyle.Render(":edit") + spacer +
		keyStyle.Render("s") + descStyle.Render(":settings") + spacer +
		keyStyle.Render("?") + descStyle.Render(":help") + spacer +
		keyStyle.Render("q") + descStyle.Render(":quit")

	keysWidth := lipgloss.Width(keys)
	if keysWidth < width {
		keys += bgStyle.Render(strings.Repeat(" ", width-keysWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, topContent, keys)
}
