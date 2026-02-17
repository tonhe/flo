package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tonhe/flo/cmd"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui"
	"github.com/tonhe/flo/tui/styles"
	"golang.org/x/term"
)

func main() {
	args := os.Args[1:]

	// Parse TUI flags (--dashboard, --theme, --help) before subcommand check
	var dashboardFlag, themeFlag string
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dashboard":
			if i+1 < len(args) {
				dashboardFlag = args[i+1]
				i++
			}
		case "--theme":
			if i+1 < len(args) {
				themeFlag = args[i+1]
				i++
			}
		case "--help", "-h":
			cmd.Execute([]string{"help"})
			return
		case "--version":
			cmd.Execute([]string{"version"})
			return
		default:
			filtered = append(filtered, args[i])
		}
	}

	// If the first non-flag argument is a known subcommand, run CLI mode
	if len(filtered) > 0 && cmd.IsSubcommand(filtered[0]) {
		cmd.Execute(filtered)
		return
	}

	// Ensure all required directories exist
	if err := config.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directories: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg := config.DefaultConfig()

	cfgDir, err := config.GetConfigDir()
	if err == nil {
		loaded, loadErr := config.LoadConfig(filepath.Join(cfgDir, "config.toml"))
		if loadErr == nil {
			cfg = loaded
		}
	}

	// Apply flag overrides
	if themeFlag != "" {
		if styles.GetThemeByName(themeFlag) != nil {
			cfg.Theme = themeFlag
		} else {
			fmt.Fprintf(os.Stderr, "Warning: unknown theme %q, using default\n", themeFlag)
		}
	}

	// Open identity store if it exists.
	// Prompt for master password unless FLO_MASTER_KEY env var is set.
	var provider identity.Provider
	storePath, err := config.GetIdentityStorePath()
	if err == nil {
		if _, statErr := os.Stat(storePath); statErr == nil {
			password := []byte(os.Getenv("FLO_MASTER_KEY"))
			if len(password) == 0 {
				fmt.Fprint(os.Stderr, "Master password: ")
				password, err = term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr) // newline after hidden input
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
					os.Exit(1)
				}
			}
			store, storeErr := identity.NewFileStore(storePath, password)
			if storeErr != nil {
				fmt.Fprintf(os.Stderr, "Error opening identity store: %v\n", storeErr)
				os.Exit(1)
			}
			provider = store
		}
	}

	mgr := engine.NewManager()
	model := tui.NewAppModel(cfg, mgr, provider, dashboardFlag)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
