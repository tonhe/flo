package styles

import (
	"testing"
)

func TestGetThemeByName(t *testing.T) {
	theme := GetThemeByName("solarized-dark")
	if theme == nil {
		t.Fatal("GetThemeByName('solarized-dark') returned nil")
	}
	if theme.Name != "Solarized Dark" {
		t.Errorf("expected name 'Solarized Dark', got %q", theme.Name)
	}
}

func TestGetThemeByNameMissing(t *testing.T) {
	theme := GetThemeByName("nonexistent")
	if theme != nil {
		t.Error("expected nil for nonexistent theme")
	}
}

func TestListThemes(t *testing.T) {
	themes := ListThemes()
	if len(themes) < 20 {
		t.Errorf("expected at least 20 themes, got %d", len(themes))
	}
}

func TestThemeCount(t *testing.T) {
	count := GetThemeCount()
	if count < 20 {
		t.Errorf("expected at least 20 themes, got %d", count)
	}
}

func TestGetThemeByIndex(t *testing.T) {
	theme := GetThemeByIndex(0)
	if theme == nil {
		t.Fatal("GetThemeByIndex(0) returned nil")
	}
}
