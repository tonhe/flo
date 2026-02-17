package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testDashboardTOML = `
name = "Test Dashboard"
default_identity = "test-v2c"
interval = "10s"
max_history = 360

[[groups]]
name = "Core"

[[groups.targets]]
host = "10.0.1.1"
label = "rtr-1"
identity = "test-v2c"
interfaces = ["Gi0/0", "Gi0/1"]

[[groups]]
name = "Branch"

[[groups.targets]]
host = "10.1.1.1"
label = "branch-1"
identity = "branch-ro"
port = 1161
interfaces = ["Eth1"]
`

func TestLoadDashboard(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.toml")
	os.WriteFile(path, []byte(testDashboardTOML), 0644)

	dash, err := LoadDashboard(path)
	if err != nil {
		t.Fatalf("LoadDashboard() error: %v", err)
	}
	if dash.Name != "Test Dashboard" {
		t.Errorf("expected name 'Test Dashboard', got %q", dash.Name)
	}
	if dash.Interval != 10*time.Second {
		t.Errorf("expected interval 10s, got %v", dash.Interval)
	}
	if len(dash.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(dash.Groups))
	}
	if len(dash.Groups[0].Targets) != 1 {
		t.Errorf("expected 1 target in group 0, got %d", len(dash.Groups[0].Targets))
	}
	if dash.Groups[1].Targets[0].Port != 1161 {
		t.Errorf("expected port 1161, got %d", dash.Groups[1].Targets[0].Port)
	}
}

func TestSaveDashboard(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.toml")

	dash := &Dashboard{
		Name:            "Saved Dashboard",
		DefaultIdentity: "prod-v3",
		Interval:        5 * time.Second,
		MaxHistory:      720,
		Groups: []Group{
			{Name: "Test", Targets: []Target{
				{Host: "1.2.3.4", Label: "test", Identity: "prod-v3", Interfaces: []string{"Eth0"}},
			}},
		},
	}
	if err := SaveDashboard(dash, path); err != nil {
		t.Fatalf("SaveDashboard() error: %v", err)
	}

	loaded, err := LoadDashboard(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if loaded.Name != "Saved Dashboard" {
		t.Errorf("expected 'Saved Dashboard', got %q", loaded.Name)
	}
}

func TestListDashboards(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.toml"), []byte(testDashboardTOML), 0644)
	os.WriteFile(filepath.Join(tmp, "b.toml"), []byte(testDashboardTOML), 0644)
	os.WriteFile(filepath.Join(tmp, "not-toml.txt"), []byte("ignore"), 0644)

	names, err := ListDashboards(tmp)
	if err != nil {
		t.Fatalf("ListDashboards() error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 dashboards, got %d", len(names))
	}
}
