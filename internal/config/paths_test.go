package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error: %v", err)
	}
	if dir == "" {
		t.Fatal("GetConfigDir() returned empty string")
	}
	// Should end with "flo"
	if filepath.Base(dir) != "flo" {
		t.Errorf("expected dir to end with 'flo', got %q", filepath.Base(dir))
	}
}

func TestGetConfigDirXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG test not applicable on Windows")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error: %v", err)
	}
	expected := filepath.Join(tmp, "flo")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestGetDataDir(t *testing.T) {
	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error: %v", err)
	}
	if dir == "" {
		t.Fatal("GetDataDir() returned empty string")
	}
	if filepath.Base(dir) != "flo" {
		t.Errorf("expected dir to end with 'flo', got %q", filepath.Base(dir))
	}
}

func TestDashboardsDir(t *testing.T) {
	dir, err := GetDashboardsDir()
	if err != nil {
		t.Fatalf("GetDashboardsDir() error: %v", err)
	}
	if filepath.Base(dir) != "dashboards" {
		t.Errorf("expected dir to end with 'dashboards', got %q", filepath.Base(dir))
	}
}
