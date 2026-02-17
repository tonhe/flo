package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "flo"

// GetConfigDir returns the platform-specific config directory.
// Unix: $XDG_CONFIG_HOME/flo or ~/.config/flo
// Windows: %APPDATA%\flo
func GetConfigDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, appName), nil
}

// GetDataDir returns the platform-specific data directory.
// Unix: $XDG_DATA_HOME/flo or ~/.local/share/flo
// Windows: %LOCALAPPDATA%\flo
func GetDataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, appName), nil
}

// GetDashboardsDir returns the directory for dashboard config files.
func GetDashboardsDir() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "dashboards"), nil
}

// GetIdentityStorePath returns the path to the encrypted identity store.
func GetIdentityStorePath() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "identities.enc"), nil
}

// EnsureDirs creates all required directories if they don't exist.
func EnsureDirs() error {
	dirs := []func() (string, error){GetConfigDir, GetDataDir, GetDashboardsDir}
	for _, fn := range dirs {
		dir, err := fn()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}
