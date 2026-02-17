package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "solarized-dark" {
		t.Errorf("expected default theme 'solarized-dark', got %q", cfg.Theme)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("expected poll interval 10s, got %v", cfg.PollInterval)
	}
	if cfg.MaxHistory != 360 {
		t.Errorf("expected max history 360, got %d", cfg.MaxHistory)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")

	cfg := DefaultConfig()
	cfg.Theme = "dracula"
	cfg.DefaultIdentity = "test-id"

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if loaded.Theme != "dracula" {
		t.Errorf("expected theme 'dracula', got %q", loaded.Theme)
	}
	if loaded.DefaultIdentity != "test-id" {
		t.Errorf("expected identity 'test-id', got %q", loaded.DefaultIdentity)
	}
}

func TestConfigLoadMissing(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadConfig() should return defaults for missing file, got error: %v", err)
	}
	if cfg.Theme != "solarized-dark" {
		t.Errorf("expected default theme, got %q", cfg.Theme)
	}
}
