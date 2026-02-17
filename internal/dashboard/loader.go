package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// LoadDashboard reads a TOML file at path and returns a populated Dashboard.
// It applies sensible defaults for missing fields: 10s interval, 360 max_history,
// port 161, and inherits default_identity for targets without an explicit identity.
func LoadDashboard(path string) (*Dashboard, error) {
	var dash Dashboard
	if _, err := toml.DecodeFile(path, &dash); err != nil {
		return nil, err
	}
	if dash.IntervalStr != "" {
		d, err := time.ParseDuration(dash.IntervalStr)
		if err == nil {
			dash.Interval = d
		}
	}
	if dash.Interval == 0 {
		dash.Interval = 10 * time.Second
	}
	if dash.MaxHistory == 0 {
		dash.MaxHistory = 360
	}
	for i := range dash.Groups {
		for j := range dash.Groups[i].Targets {
			if dash.Groups[i].Targets[j].Port == 0 {
				dash.Groups[i].Targets[j].Port = 161
			}
			if dash.Groups[i].Targets[j].Identity == "" {
				dash.Groups[i].Targets[j].Identity = dash.DefaultIdentity
			}
		}
	}
	return &dash, nil
}

// SaveDashboard writes a Dashboard to a TOML file at path.
// It serialises the Interval duration into IntervalStr before encoding.
func SaveDashboard(dash *Dashboard, path string) error {
	dash.IntervalStr = dash.Interval.String()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(dash)
}

// ListDashboards returns the base names (without .toml extension) of all TOML
// files found in dir.
func ListDashboards(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			names = append(names, name)
		}
	}
	return names, nil
}
