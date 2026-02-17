package styles

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// Theme represents a Base16 color scheme.
type Theme struct {
	Name   string
	Base00 lipgloss.Color // Background
	Base01 lipgloss.Color // Lighter background
	Base02 lipgloss.Color // Selection
	Base03 lipgloss.Color // Comments / dim
	Base04 lipgloss.Color // Light foreground
	Base05 lipgloss.Color // Foreground
	Base06 lipgloss.Color // Light foreground
	Base07 lipgloss.Color // Light background
	Base08 lipgloss.Color // Red
	Base09 lipgloss.Color // Orange
	Base0A lipgloss.Color // Yellow
	Base0B lipgloss.Color // Green
	Base0C lipgloss.Color // Cyan
	Base0D lipgloss.Color // Blue
	Base0E lipgloss.Color // Magenta
	Base0F lipgloss.Color // Brown
}

var (
	DefaultTheme Theme
	sortedSlugs  []string
)

func init() {
	sortedSlugs = make([]string, 0, len(Themes))
	for slug := range Themes {
		sortedSlugs = append(sortedSlugs, slug)
	}
	sort.Strings(sortedSlugs)
	DefaultTheme = Themes["solarized-dark"]
}

// SetTheme updates the default theme.
func SetTheme(theme Theme) {
	DefaultTheme = theme
}

// GetThemeByName returns a theme by its slug, or nil if not found.
func GetThemeByName(name string) *Theme {
	t, ok := Themes[name]
	if !ok {
		return nil
	}
	return &t
}

// ListThemes returns sorted theme slugs.
func ListThemes() []string {
	return sortedSlugs
}

// GetThemeCount returns the total number of available themes.
func GetThemeCount() int {
	return len(Themes)
}

// GetThemeByIndex returns a theme at the given sorted index.
func GetThemeByIndex(idx int) *Theme {
	if idx < 0 || idx >= len(sortedSlugs) {
		return nil
	}
	t := Themes[sortedSlugs[idx]]
	return &t
}

// GetThemeIndex returns the sorted index of a theme slug, or -1.
func GetThemeIndex(slug string) int {
	for i, s := range sortedSlugs {
		if s == slug {
			return i
		}
	}
	return -1
}
