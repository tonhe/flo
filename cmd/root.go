package cmd

import (
	"fmt"
	"os"
)

// knownSubcommands is the set of CLI subcommands that bypass the TUI.
var knownSubcommands = map[string]bool{
	"identity": true,
	"discover": true,
	"config":   true,
	"themes":   true,
	"version":  true,
	"help":     true,
}

// IsSubcommand returns true if the argument is a known CLI subcommand.
func IsSubcommand(arg string) bool {
	return knownSubcommands[arg]
}

// Execute dispatches to the appropriate CLI subcommand handler.
func Execute(args []string) {
	if len(args) == 0 {
		return
	}

	switch args[0] {
	case "identity":
		identityCmd(args[1:])
	case "discover":
		discoverCmd(args[1:])
	case "config":
		configCmd(args[1:])
	case "themes":
		themesCmd()
	case "version":
		fmt.Println("flo v0.1.0")
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`flo - SNMP interface monitor

Usage:
  flo                       Launch TUI monitor
  flo --dashboard NAME      Launch with specific dashboard
  flo --theme NAME          Launch with theme override
  flo identity <cmd>        Manage SNMP identities
  flo discover HOST         Discover device interfaces
  flo config <cmd>          Manage configuration
  flo themes                List available themes
  flo version               Show version
  flo help                  Show this help

Identity Commands:
  flo identity list                List all identities
  flo identity add                 Add a new identity (interactive)
  flo identity remove NAME         Remove an identity
  flo identity test NAME HOST      Test SNMP connectivity

Discovery:
  flo discover --identity NAME HOST   Discover interfaces on a device

Config Commands:
  flo config path                  Show config directory path
  flo config theme NAME            Set default theme
  flo config identity NAME         Set default identity`)
}
