package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/tui"
)

func main() {
	cfg := config.DefaultConfig()

	cfgDir, err := config.GetConfigDir()
	if err == nil {
		loaded, loadErr := config.LoadConfig(filepath.Join(cfgDir, "config.toml"))
		if loadErr == nil {
			cfg = loaded
		}
	}

	mgr := engine.NewManager()
	model := tui.NewAppModel(cfg, mgr, nil)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
