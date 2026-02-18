package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/config"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/internal/identity"
	"golang.org/x/term"
)

func identityCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: flo identity <list|add|remove|test>")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		identityList()
	case "add":
		identityAdd()
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: flo identity remove NAME")
			os.Exit(1)
		}
		identityRemove(args[1])
	case "test":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: flo identity test NAME HOST")
			os.Exit(1)
		}
		identityTest(args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown identity command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: flo identity <list|add|remove|test>")
		os.Exit(1)
	}
}

// openStore opens the identity store, prompting for the master password if needed.
// Tries empty password first to support no-password vaults.
func openStore() *identity.FileStore {
	storePath, err := config.GetIdentityStorePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := config.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directories: %v\n", err)
		os.Exit(1)
	}

	// Try empty password first (no-password vault)
	store, storeErr := identity.NewFileStore(storePath, []byte(""))
	if storeErr == nil {
		return store
	}

	// Empty password didn't work, try env var or prompt
	password := getMasterPassword()
	store, storeErr = identity.NewFileStore(storePath, password)
	if storeErr != nil {
		fmt.Fprintf(os.Stderr, "Error opening identity store: %v\n", storeErr)
		os.Exit(1)
	}
	return store
}

// getMasterPassword reads the master password from FLO_MASTER_KEY env var or prompts.
func getMasterPassword() []byte {
	if key := os.Getenv("FLO_MASTER_KEY"); key != "" {
		return []byte(key)
	}

	fmt.Fprint(os.Stderr, "Master password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after password input
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
		os.Exit(1)
	}
	return password
}

func identityList() {
	store := openStore()
	summaries, err := store.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing identities: %v\n", err)
		os.Exit(1)
	}

	if len(summaries) == 0 {
		fmt.Println("No identities configured.")
		return
	}

	for _, s := range summaries {
		line := fmt.Sprintf("%-20s  version=%s", s.Name, s.Version)
		if s.Username != "" {
			line += fmt.Sprintf("  user=%s", s.Username)
		}
		if s.AuthProto != "" {
			line += fmt.Sprintf("  auth=%s", s.AuthProto)
		}
		if s.PrivProto != "" {
			line += fmt.Sprintf("  priv=%s", s.PrivProto)
		}
		fmt.Println(line)
	}
}

func identityAdd() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Identity name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		fmt.Fprintln(os.Stderr, "Error: name is required")
		os.Exit(1)
	}

	fmt.Print("SNMP version (1, 2c, 3): ")
	version, _ := reader.ReadString('\n')
	version = strings.TrimSpace(version)
	if version != "1" && version != "2c" && version != "3" {
		fmt.Fprintln(os.Stderr, "Error: version must be 1, 2c, or 3")
		os.Exit(1)
	}

	id := identity.Identity{
		Name:    name,
		Version: version,
	}

	switch version {
	case "1", "2c":
		fmt.Print("Community string: ")
		community, _ := reader.ReadString('\n')
		id.Community = strings.TrimSpace(community)
		if id.Community == "" {
			fmt.Fprintln(os.Stderr, "Error: community string is required for v1/v2c")
			os.Exit(1)
		}

	case "3":
		fmt.Print("Username: ")
		username, _ := reader.ReadString('\n')
		id.Username = strings.TrimSpace(username)
		if id.Username == "" {
			fmt.Fprintln(os.Stderr, "Error: username is required for v3")
			os.Exit(1)
		}

		fmt.Print("Auth protocol (none, MD5, SHA, SHA256, SHA512): ")
		authProto, _ := reader.ReadString('\n')
		authProto = strings.TrimSpace(authProto)
		if authProto != "" && authProto != "none" {
			id.AuthProto = strings.ToUpper(authProto)

			fmt.Fprint(os.Stderr, "Auth password: ")
			authPass, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}
			id.AuthPass = string(authPass)

			fmt.Print("Privacy protocol (none, DES, AES128, AES192, AES256): ")
			privProto, _ := reader.ReadString('\n')
			privProto = strings.TrimSpace(privProto)
			if privProto != "" && privProto != "none" {
				id.PrivProto = strings.ToUpper(privProto)

				fmt.Fprint(os.Stderr, "Privacy password: ")
				privPass, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
					os.Exit(1)
				}
				id.PrivPass = string(privPass)
			}
		}
	}

	store := openStore()
	if err := store.Add(id); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding identity: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Identity %q added.\n", name)
}

func identityRemove(name string) {
	store := openStore()
	if err := store.Remove(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing identity: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Identity %q removed.\n", name)
}

func identityTest(name, host string) {
	store := openStore()
	id, err := store.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Testing SNMP connectivity to %s using identity %q...\n", host, name)

	client, err := engine.NewSNMPClient(host, 161, id, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating SNMP client: %v\n", err)
		os.Exit(1)
	}

	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %v\n", host, err)
		os.Exit(1)
	}
	defer client.Conn.Close()

	// GET sysDescr.0
	oid := "1.3.6.1.2.1.1.1.0"
	result, err := client.Get([]string{oid})
	if err != nil {
		fmt.Fprintf(os.Stderr, "SNMP GET failed: %v\n", err)
		os.Exit(1)
	}

	for _, pdu := range result.Variables {
		switch pdu.Type {
		case gosnmp.OctetString:
			fmt.Printf("sysDescr: %s\n", string(pdu.Value.([]byte)))
		default:
			fmt.Printf("sysDescr: %v\n", pdu.Value)
		}
	}

	fmt.Println("Connection test successful.")
}

