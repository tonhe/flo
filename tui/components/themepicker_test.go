package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tonhe/flo/tui/styles"
)

func TestNewThemePickerModel(t *testing.T) {
	m := NewThemePickerModel("solarized-dark")
	expected := styles.GetThemeIndex("solarized-dark")
	if expected < 0 {
		t.Fatal("solarized-dark not found in theme list")
	}
	if m.cursor != expected {
		t.Errorf("cursor = %d, want %d", m.cursor, expected)
	}
	if m.initialIndex != expected {
		t.Errorf("initialIndex = %d, want %d", m.initialIndex, expected)
	}
}

func TestNewThemePickerModelUnknownSlug(t *testing.T) {
	m := NewThemePickerModel("nonexistent-theme-slug")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 for unknown slug", m.cursor)
	}
	if m.initialIndex != 0 {
		t.Errorf("initialIndex = %d, want 0 for unknown slug", m.initialIndex)
	}
}

func TestThemePickerPreviewTheme(t *testing.T) {
	m := NewThemePickerModel("solarized-dark")
	theme := m.PreviewTheme()
	expected := styles.GetThemeByName("solarized-dark")
	if expected == nil {
		t.Fatal("solarized-dark theme not found")
	}
	if theme.Name != expected.Name {
		t.Errorf("PreviewTheme().Name = %q, want %q", theme.Name, expected.Name)
	}
}

func TestThemePickerNavigation(t *testing.T) {
	m := NewThemePickerModel(styles.ListThemes()[0])
	m.SetSize(120, 40)

	if m.cursor != 0 {
		t.Fatalf("expected cursor to start at 0, got %d", m.cursor)
	}

	// Down should increment cursor.
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	m, _, action := m.Update(downMsg)
	if action != ThemePickerNone {
		t.Errorf("Down action = %d, want ThemePickerNone", action)
	}
	if m.cursor != 1 {
		t.Errorf("after Down: cursor = %d, want 1", m.cursor)
	}

	// Up should decrement cursor.
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	m, _, action = m.Update(upMsg)
	if action != ThemePickerNone {
		t.Errorf("Up action = %d, want ThemePickerNone", action)
	}
	if m.cursor != 0 {
		t.Errorf("after Up: cursor = %d, want 0", m.cursor)
	}
}

func TestThemePickerBounds(t *testing.T) {
	themes := styles.ListThemes()
	m := NewThemePickerModel(themes[0])
	m.SetSize(120, 40)

	// Pressing Up at cursor 0 should stay at 0.
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	m, _, _ = m.Update(upMsg)
	if m.cursor != 0 {
		t.Errorf("cursor went below 0: got %d", m.cursor)
	}

	// Move cursor to the last theme.
	maxIdx := len(themes) - 1
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	for i := 0; i < maxIdx; i++ {
		m, _, _ = m.Update(downMsg)
	}
	if m.cursor != maxIdx {
		t.Fatalf("cursor = %d, want %d after navigating to end", m.cursor, maxIdx)
	}

	// Pressing Down at the last index should stay at max.
	m, _, _ = m.Update(downMsg)
	if m.cursor != maxIdx {
		t.Errorf("cursor went above max: got %d, want %d", m.cursor, maxIdx)
	}
}

func TestThemePickerSelect(t *testing.T) {
	m := NewThemePickerModel("solarized-dark")
	m.SetSize(120, 40)

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, _, action := m.Update(enterMsg)
	if action != ThemePickerSelected {
		t.Errorf("Enter action = %d, want ThemePickerSelected (%d)", action, ThemePickerSelected)
	}
}

func TestThemePickerCancel(t *testing.T) {
	m := NewThemePickerModel("solarized-dark")
	m.SetSize(120, 40)

	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	_, _, action := m.Update(escMsg)
	if action != ThemePickerCancelled {
		t.Errorf("Esc action = %d, want ThemePickerCancelled (%d)", action, ThemePickerCancelled)
	}
}

func TestThemePickerSelectedSlug(t *testing.T) {
	themes := styles.ListThemes()
	m := NewThemePickerModel(themes[0])
	m.SetSize(120, 40)

	// At cursor 0, SelectedSlug should return the first theme.
	if slug := m.SelectedSlug(); slug != themes[0] {
		t.Errorf("SelectedSlug() = %q, want %q", slug, themes[0])
	}

	// Move down one and check again.
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	m, _, _ = m.Update(downMsg)
	if slug := m.SelectedSlug(); slug != themes[1] {
		t.Errorf("after Down: SelectedSlug() = %q, want %q", slug, themes[1])
	}
}

func TestThemePickerViewNotEmpty(t *testing.T) {
	m := NewThemePickerModel("solarized-dark")
	m.SetSize(120, 40)
	v := m.View()
	if len(v) == 0 {
		t.Error("View() returned empty string")
	}
}

func TestThemePickerScrollAdjustment(t *testing.T) {
	m := NewThemePickerModel(styles.ListThemes()[0])
	// Set a very small height so only a few rows are visible.
	// height of 8 means contentHeight = 8 - 5 = 3 visible rows.
	m.SetSize(120, 8)

	if m.scrollOffset != 0 {
		t.Fatalf("initial scrollOffset = %d, want 0", m.scrollOffset)
	}

	// Move the cursor down past the visible area.
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	themeCount := len(styles.ListThemes())
	steps := 6
	if steps > themeCount-1 {
		steps = themeCount - 1
	}
	for i := 0; i < steps; i++ {
		m, _, _ = m.Update(downMsg)
	}

	if m.scrollOffset <= 0 {
		t.Errorf("scrollOffset = %d after moving cursor to %d with small height; expected > 0", m.scrollOffset, m.cursor)
	}
}
