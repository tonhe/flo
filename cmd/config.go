package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/tui/styles"
)

func configCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: flo config <path|theme|identity>")
		os.Exit(1)
	}

	switch args[0] {
	case "path":
		configPath()
	case "theme":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: flo config theme NAME")
			os.Exit(1)
		}
		configSetTheme(args[1])
	case "identity":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: flo config identity NAME")
			os.Exit(1)
		}
		configSetIdentity(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: flo config <path|theme|identity>")
		os.Exit(1)
	}
}

func configPath() {
	dir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(dir)
}

func configSetTheme(name string) {
	// Validate the theme name exists
	if styles.GetThemeByName(name) == nil {
		fmt.Fprintf(os.Stderr, "Error: unknown theme %q\n", name)
		fmt.Fprintln(os.Stderr, "Run 'flo themes' to see available themes.")
		os.Exit(1)
	}

	cfg := loadOrDefaultConfig()
	cfg.Theme = name
	saveConfig(cfg)

	fmt.Printf("Default theme set to %q.\n", name)
}

func configSetIdentity(name string) {
	cfg := loadOrDefaultConfig()
	cfg.DefaultIdentity = name
	saveConfig(cfg)

	fmt.Printf("Default identity set to %q.\n", name)
}

func themesCmd() {
	for _, name := range styles.ListThemes() {
		fmt.Println(name)
	}
}

// loadOrDefaultConfig loads the config from disk, falling back to defaults.
func loadOrDefaultConfig() *config.Config {
	cfgDir, err := config.GetConfigDir()
	if err != nil {
		return config.DefaultConfig()
	}
	cfg, err := config.LoadConfig(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// saveConfig writes the config to disk, creating directories as needed.
func saveConfig(cfg *config.Config) {
	if err := config.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directories: %v\n", err)
		os.Exit(1)
	}

	cfgDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := config.SaveConfig(cfg, filepath.Join(cfgDir, "config.toml")); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}
}
