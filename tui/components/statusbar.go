package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

// RenderStatusBar renders the two-line status/footer bar showing poll info,
// health status, and key bindings.
func RenderStatusBar(theme styles.Theme, interval time.Duration, lastPoll time.Time, okCount, totalCount, width int) string {
	pollInfo := fmt.Sprintf("poll: %s", interval)
	lastStr := "never"
	if !lastPoll.IsZero() {
		lastStr = lastPoll.Format("15:04:05")
	}
	healthColor := theme.Base0B
	if okCount < totalCount {
		healthColor = theme.Base0A
	}

	stats := fmt.Sprintf("%s | last: %s | ", pollInfo, lastStr)
	health := lipgloss.NewStyle().Foreground(healthColor).
		Render(fmt.Sprintf("%d/%d OK", okCount, totalCount))

	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.Base04)

	keys := fmt.Sprintf(
		" %s:%s  %s:%s  %s:%s  %s:%s  %s:%s  %s:%s  %s:%s",
		keyStyle.Render("enter"), descStyle.Render("detail"),
		keyStyle.Render("d"), descStyle.Render("dashboards"),
		keyStyle.Render("i"), descStyle.Render("identities"),
		keyStyle.Render("n"), descStyle.Render("new"),
		keyStyle.Render("s"), descStyle.Render("settings"),
		keyStyle.Render("?"), descStyle.Render("help"),
		keyStyle.Render("q"), descStyle.Render("quit"),
	)

	top := lipgloss.NewStyle().Background(theme.Base01).Width(width).
		Render(fmt.Sprintf(" %s%s", stats, health))
	bottom := lipgloss.NewStyle().Background(theme.Base01).Width(width).
		Render(keys)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}
