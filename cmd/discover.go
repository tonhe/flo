package cmd

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/tonhe/flo/internal/engine"
)

func discoverCmd(args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	identityName := fs.String("identity", "", "Identity name to use for SNMP authentication")
	port := fs.Int("port", 161, "SNMP port")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: flo discover --identity NAME [--port PORT] HOST")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *identityName == "" {
		fmt.Fprintln(os.Stderr, "Error: --identity is required")
		fs.Usage()
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: HOST argument is required")
		fs.Usage()
		os.Exit(1)
	}

	host := fs.Arg(0)

	store := openStore()
	id, err := store.Get(*identityName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Discovering interfaces on %s...\n", host)

	interfaces, err := engine.DiscoverInterfaces(host, *port, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering interfaces: %v\n", err)
		os.Exit(1)
	}

	if len(interfaces) == 0 {
		fmt.Println("No interfaces found.")
		return
	}

	// Sort by interface index for consistent output
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].IfIndex < interfaces[j].IfIndex
	})

	fmt.Printf("Found %d interfaces on %s:\n\n", len(interfaces), host)
	fmt.Printf("%-6s  %-6s  %-30s  %-40s  %10s  %s\n", "Index", "Status", "Name", "Description", "Speed", "Alias")
	fmt.Printf("%-6s  %-6s  %-30s  %-40s  %10s  %s\n", "-----", "------", "----", "-----------", "-----", "-----")

	for _, iface := range interfaces {
		speedStr := ""
		if iface.Speed > 0 {
			speedStr = formatSpeed(iface.Speed)
		}
		fmt.Printf("%-6d  %-6s  %-30s  %-40s  %10s  %s\n",
			iface.IfIndex,
			iface.Status,
			truncate(iface.Name, 30),
			truncate(iface.Description, 40),
			speedStr,
			iface.Alias,
		)
	}
}

// formatSpeed formats a speed in Mbps to a human-readable string.
func formatSpeed(mbps uint64) string {
	switch {
	case mbps >= 1000000:
		return fmt.Sprintf("%d Tbps", mbps/1000000)
	case mbps >= 1000:
		return fmt.Sprintf("%d Gbps", mbps/1000)
	default:
		return fmt.Sprintf("%d Mbps", mbps)
	}
}

// truncate shortens a string to the given max length, adding "..." if needed.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
