package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "flo - SNMP interface monitor")
	os.Exit(0)
}
