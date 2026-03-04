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

// NeedsRestart returns true if the new dashboard differs from the old one
// in ways that require stopping and restarting the polling engine (e.g.
// interval, hosts, interfaces, identities). Cosmetic changes like labels
// do not require a restart.
func NeedsRestart(old, new *Dashboard) bool {
	if old.Name != new.Name {
		return true
	}
	if old.Interval != new.Interval {
		return true
	}
	if old.DefaultIdentity != new.DefaultIdentity {
		return true
	}
	if old.MaxHistory != new.MaxHistory {
		return true
	}
	if len(old.Groups) != len(new.Groups) {
		return true
	}
	for i := range old.Groups {
		og, ng := old.Groups[i], new.Groups[i]
		if og.Name != ng.Name || len(og.Targets) != len(ng.Targets) {
			return true
		}
		for j := range og.Targets {
			ot, nt := og.Targets[j], ng.Targets[j]
			if ot.Host != nt.Host || ot.Port != nt.Port || ot.Identity != nt.Identity {
				return true
			}
			if len(ot.Interfaces) != len(nt.Interfaces) {
				return true
			}
			for k := range ot.Interfaces {
				if ot.Interfaces[k] != nt.Interfaces[k] {
					return true
				}
			}
		}
	}
	return false
}
