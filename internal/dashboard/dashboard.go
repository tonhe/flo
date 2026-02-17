package dashboard

import "time"

// Dashboard represents a complete dashboard configuration loaded from TOML.
type Dashboard struct {
	Name            string        `toml:"name"`
	DefaultIdentity string        `toml:"default_identity"`
	IntervalStr     string        `toml:"interval"`
	Interval        time.Duration `toml:"-"`
	MaxHistory      int           `toml:"max_history"`
	Groups          []Group       `toml:"groups"`
}

// Group represents a named collection of monitoring targets.
type Group struct {
	Name    string   `toml:"name"`
	Targets []Target `toml:"targets"`
}

// Target represents a single SNMP device to monitor.
type Target struct {
	Host       string   `toml:"host"`
	Label      string   `toml:"label"`
	Identity   string   `toml:"identity"`
	Port       int      `toml:"port"`
	Interfaces []string `toml:"interfaces"`
}
