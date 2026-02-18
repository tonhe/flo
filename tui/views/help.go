package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

// HelpView renders a modal overlay showing all keyboard shortcuts.
type HelpView struct {
	theme   styles.Theme
	sty     *styles.Styles
	width   int
	height  int
	visible bool
}

// NewHelpView creates a new HelpView with the given theme.
func NewHelpView(theme styles.Theme) HelpView {
	return HelpView{
		theme: theme,
		sty:   styles.NewStyles(theme),
	}
}

// Toggle flips the help overlay visibility.
func (v *HelpView) Toggle() {
	v.visible = !v.visible
}

// IsVisible returns whether the help overlay is currently shown.
func (v HelpView) IsVisible() bool {
	return v.visible
}

// SetSize updates the available dimensions for the overlay.
func (v *HelpView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// View renders the help overlay as a centered modal box.
func (v HelpView) View() string {
	modalWidth := 48
	if v.width > 60 {
		modalWidth = v.width / 2
		if modalWidth > 56 {
			modalWidth = 56
		}
	}
	if modalWidth < 38 {
		modalWidth = 38
	}

	innerWidth := modalWidth - 6 // border + padding

	sectionStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0E).
		Bold(true)
	keyStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base05)
	dimStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base04)

	// Helper to format a keybinding line with aligned columns.
	bindingLine := func(keys, desc string) string {
		return fmt.Sprintf("  %s  %s",
			keyStyle.Render(padRight(keys, 16)),
			descStyle.Render(desc),
		)
	}

	var lines []string

	// Global section
	lines = append(lines, sectionStyle.Render("Global"))
	lines = append(lines, bindingLine("Ctrl+C", "Quit"))
	lines = append(lines, bindingLine("?", "Toggle this help"))
	lines = append(lines, "")

	// Dashboard section
	lines = append(lines, sectionStyle.Render("Dashboard"))
	lines = append(lines, bindingLine("q", "Quit"))
	lines = append(lines, bindingLine("Up / Down", "Navigate interfaces"))
	lines = append(lines, bindingLine("Enter", "Detail view"))
	lines = append(lines, bindingLine("d", "Dashboard switcher"))
	lines = append(lines, bindingLine("e", "Edit active dashboard"))
	lines = append(lines, bindingLine("i", "Identity manager"))
	lines = append(lines, bindingLine("s", "Settings"))
	lines = append(lines, bindingLine("r", "Force refresh"))
	lines = append(lines, "")

	// Switcher section
	lines = append(lines, sectionStyle.Render("Dashboard Switcher"))
	lines = append(lines, bindingLine("Enter", "Switch to dashboard"))
	lines = append(lines, bindingLine("n", "New dashboard"))
	lines = append(lines, bindingLine("e", "Edit dashboard"))
	lines = append(lines, bindingLine("x", "Stop engine"))
	lines = append(lines, bindingLine("Esc", "Close"))
	lines = append(lines, "")

	// Detail View section
	lines = append(lines, sectionStyle.Render("Detail View"))
	lines = append(lines, bindingLine("Esc", "Back to dashboard"))
	lines = append(lines, "")

	// Identity / Builder section
	lines = append(lines, sectionStyle.Render("Identity / Builder"))
	lines = append(lines, bindingLine("Esc", "Close / Back"))
	lines = append(lines, bindingLine("Enter", "Select / Confirm"))
	lines = append(lines, "")

	// Footer hint
	lines = append(lines, dimStyle.Render("[?] close"))

	content := strings.Join(lines, "\n")

	// Modal box with rounded border
	modal := v.sty.ModalBorder.
		Width(innerWidth).
		Render(content)

	// Place title into the top border
	title := v.sty.ModalTitle.Render(" Keyboard Shortcuts ")
	modalLines := strings.Split(modal, "\n")
	if len(modalLines) > 0 {
		borderLine := modalLines[0]
		if len(borderLine) > 2 {
			runes := []rune(borderLine)
			titleRunes := []rune(title)
			insertPos := 2
			if insertPos+len(titleRunes) < len(runes) {
				combined := make([]rune, 0, len(runes))
				combined = append(combined, runes[:insertPos]...)
				combined = append(combined, titleRunes...)
				combined = append(combined, runes[insertPos+len(titleRunes):]...)
				modalLines[0] = string(combined)
			}
		}
		modal = strings.Join(modalLines, "\n")
	}

	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, modal)
}
