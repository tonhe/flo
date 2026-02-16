# flo — TUI SNMP Interface Monitor

## Overview

flo is a real-time SNMP interface monitoring tool with a terminal UI built in Go using Bubble Tea. It provides identity management for SNMP credentials, dashboard management for organizing monitored interfaces, and concurrent polling engines that keep dashboards running in the background while you switch between views.

**Inspiration**: [trafikwatch](https://github.com/scottpeterman/trafikwatch) (SNMP monitoring, sparklines, split-screen) + [nbor](https://github.com/tonhe/nbor) (Go/Bubble Tea TUI patterns, Base16 themes, state machine architecture).

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Consistent with nbor, fast binary, great concurrency |
| TUI Framework | Bubble Tea + Lipgloss + Bubbles | Charm ecosystem, same as nbor |
| SNMP Library | gosnmp | Most popular Go SNMP lib, pure Go, used in Telegraf |
| Config Format | TOML | Human-editable, same as nbor |
| Credential Storage | AES-256-GCM encrypted file | Portable across all OS, no keyring dependency |
| Theme System | Base16 (20+ themes) | Consistent with nbor, community standard |
| Architecture | Hybrid monolithic with concurrent engines | Single binary simplicity, background polling via goroutines |

## Architecture

Single binary, single process. SNMP polling runs as goroutines managed by an EngineManager. The TUI layer depends on internal packages but not vice versa. All inter-component communication via Go channels and Bubble Tea messages.

```
┌─ flo process ──────────────────────────────────────────┐
│  Bubble Tea TUI                                        │
│    ├─ Dashboard View (shows active dashboard)          │
│    ├─ Dashboard Switcher (list all, start/stop)        │
│    ├─ Identity Manager (CRUD credentials)              │
│    ├─ Dashboard Builder Wizard                         │
│    └─ Settings View                                    │
│                                                        │
│  Engine Manager                                        │
│    ├─ Engine A: "WAN Links" (polling, viewed)          │
│    ├─ Engine B: "DC Fabric" (polling, background)      │
│    └─ Engine C: "Branch" (stopped)                     │
│                                                        │
│  Shared Services                                       │
│    ├─ Identity Provider (encrypted store)              │
│    └─ Config Manager (TOML)                            │
│                                                        │
│  Clean interfaces → extractable to daemon later        │
└────────────────────────────────────────────────────────┘
```

## Package Structure

```
flo/
├── main.go                      # CLI entry point, flag parsing, app bootstrap
├── cmd/
│   └── root.go                  # CLI commands: monitor, discover, identity, config
│
├── internal/
│   ├── engine/                  # SNMP polling engine (core business logic)
│   │   ├── manager.go           # EngineManager: start/stop/list dashboard engines
│   │   ├── poller.go            # Per-dashboard polling loop (goroutine)
│   │   ├── snmp.go              # gosnmp wrapper: GET, WALK, BULK operations
│   │   ├── discover.go          # Device interface discovery (walk ifName/ifDescr/etc)
│   │   ├── rate.go              # Rate calculation, counter wrap detection
│   │   └── ringbuffer.go        # Fixed-size sample history per interface
│   │
│   ├── identity/                # Credential management
│   │   ├── provider.go          # IdentityProvider interface
│   │   ├── store.go             # Encrypted file store (AES-256-GCM)
│   │   ├── identity.go          # Identity types: v2c community, v3 credentials
│   │   └── crypto.go            # Key derivation (Argon2id), encrypt/decrypt helpers
│   │
│   ├── dashboard/               # Dashboard config & lifecycle
│   │   ├── dashboard.go         # Dashboard model: groups, targets, interfaces
│   │   ├── loader.go            # Load/save TOML dashboard configs
│   │   └── registry.go          # Track all known dashboards + engine state
│   │
│   ├── config/                  # App-level configuration
│   │   ├── config.go            # Global settings (theme, log dir, defaults)
│   │   └── paths.go             # XDG-compliant config/data paths per platform
│   │
│   └── types/                   # Shared data types
│       ├── models.go            # InterfaceStats, RateSample, DeviceInfo
│       └── events.go            # Inter-component message types
│
├── tui/                         # Bubble Tea TUI layer
│   ├── app.go                   # Root AppModel: state machine, message routing
│   ├── views/
│   │   ├── dashboard.go         # Main monitoring view (tables, sparklines)
│   │   ├── dashboard_detail.go  # Split-screen detail with charts
│   │   ├── switcher.go          # Dashboard list/switcher overlay
│   │   ├── identity.go          # Identity CRUD view
│   │   ├── builder.go           # Dashboard builder wizard
│   │   ├── discover.go          # Device discovery results view
│   │   └── settings.go          # Settings/config editor
│   ├── components/
│   │   ├── table.go             # Reusable data table with sorting/scrolling
│   │   ├── sparkline.go         # Unicode block sparkline renderer
│   │   ├── chart.go             # ASCII line chart for detail view
│   │   ├── statusbar.go         # Bottom status bar
│   │   ├── header.go            # Top header bar
│   │   ├── modal.go             # Floating modal/dialog
│   │   └── form.go              # Form inputs for wizard/settings
│   ├── styles/
│   │   ├── styles.go            # Lipgloss style definitions
│   │   ├── theme.go             # Base16 theme struct + loading
│   │   └── themes_data.go       # 20+ built-in theme definitions
│   └── keys/
│       └── keys.go              # Key binding definitions
│
├── docs/plans/                  # Design & planning docs
└── go.mod
```

### Key Separation Principles

- `internal/engine` knows nothing about the TUI — pure business logic
- `internal/identity` knows nothing about SNMP — just credential storage
- `tui/` depends on `internal/` but not vice versa
- EngineManager is the central coordinator: TUI talks to it, it manages pollers

## Identity Management

### Encrypted Store

Identities are named credential profiles stored in an AES-256-GCM encrypted file (`~/.config/flo/identities.enc`). Key is derived via Argon2id from a master password.

```
"lab-v2c"    → { version: "2c", community: "lab" }
"prod-v3"    → { version: "3", username: "monitor", auth: SHA256, priv: AES256 }
"branch-ro"  → { version: "2c", community: "public" }
```

### Identity Defaults (creation-time only)

- **Global default identity**: set in app config, pre-fills the identity field when adding targets
- **Dashboard default identity**: set per dashboard, overrides global default for that dashboard
- **When a target is saved**: the resolved identity name is baked into the target config

Defaults are UX sugar for the builder wizard. They never participate in runtime resolution. Every saved target has an explicit `identity = "name"` field. Dashboards are self-contained.

### Master Password Flow

1. First run: prompt to create master password → derive key → store salt
2. Subsequent runs: prompt for master password → derive key → decrypt store
3. Env var `FLO_MASTER_KEY` bypasses prompt (for automation)
4. File locked with OS file lock to prevent corruption

### IdentityProvider Interface

```go
type IdentityProvider interface {
    List() ([]IdentitySummary, error)
    Get(name string) (*Identity, error)
    Add(identity Identity) error
    Update(name string, identity Identity) error
    Remove(name string) error
    Test(name string, host string, port int) error
}
```

Implemented by encrypted file store. Interface allows future backends (OS keyring, HashiCorp Vault).

## SNMP Engine

### EngineManager

```go
type EngineManager struct {
    // Start/stop/list dashboard polling engines
    StartDashboard(name string, dash Dashboard, identities IdentityProvider) error
    StopDashboard(name string) error
    GetDashboardData(name string) (*DashboardSnapshot, error)
    ListEngines() []EngineInfo  // name, state, lastPoll, errorCount
    Subscribe(name string) <-chan EngineEvent  // for TUI updates
}
```

### Poller (per dashboard goroutine)

1. **Init**: resolve identity names → build gosnmp client configs per target
2. **Discovery**: walk ifName, ifDescr, ifAlias, ifHighSpeed for each target (concurrent)
3. **Poll loop** (configurable interval, default 10s):
   - For each target (concurrent via goroutines):
     - GET ifHCInOctets, ifHCOutOctets, ifHighSpeed, ifOperStatus
     - Calculate rates (bits/sec), detect counter wraps
     - Append RateSample to ring buffer (default 360 = 1hr at 10s)
   - Publish EngineEvent to subscribers
4. **Shutdown**: close gosnmp connections

### Rate Calculation

```
delta_in = current_in_octets - previous_in_octets
if delta_in < 0 → counter reset, skip sample
in_rate_bps = (delta_in * 8) / elapsed_seconds
utilization = max(in_rate, out_rate) / (ifHighSpeed * 1_000_000) * 100
```

### Standard IF-MIB OIDs

- ifName (1.3.6.1.2.1.31.1.1.1.1)
- ifDescr (1.3.6.1.2.1.2.2.1.2) — fallback
- ifAlias (1.3.6.1.2.1.31.1.1.1.18) — user description
- ifHCInOctets (1.3.6.1.2.1.31.1.1.1.6) — 64-bit in counter
- ifHCOutOctets (1.3.6.1.2.1.31.1.1.1.10) — 64-bit out counter
- ifHighSpeed (1.3.6.1.2.1.31.1.1.1.15) — speed in Mbps
- ifOperStatus (1.3.6.1.2.1.2.2.1.8) — up/down/testing

## Dashboard Configuration

### TOML Format

```toml
name = "WAN Links"
default_identity = "prod-v3"
interval = "10s"
max_history = 360

[[groups]]
name = "WAN Core"

[[groups.targets]]
host = "10.0.1.1"
label = "core-rtr-1"
identity = "prod-v3"
interfaces = ["Gi0/0", "Gi0/1"]

[[groups.targets]]
host = "10.0.2.1"
label = "core-rtr-2"
identity = "lab-v2c"
port = 1161
interfaces = ["Ethernet1", "Ethernet2"]

[[groups]]
name = "Branch Sites"

[[groups.targets]]
host = "10.1.1.1"
label = "branch-01"
identity = "branch-ro"
interfaces = ["Eth1"]
```

### Dashboard Lifecycle

- Stored as individual TOML files in `~/.config/flo/dashboards/`
- Created via TUI builder wizard or by placing TOML files directly
- Referenced by filename (without `.toml` extension)
- Dashboard registry tracks all known dashboards + engine state (running/stopped)

## TUI Design

### State Machine

```
Startup (master password) → Dashboard Monitor (main)
  ↕ Dashboard Switcher (overlay)
  ↕ Detail View (split-screen)
  ↕ Identity Manager (full-screen)
  ↕ Dashboard Builder (wizard)
  ↕ Settings (full-screen)
```

### Main Dashboard View

```
┌─ flo ──────────────────── WAN Links ─── ● LIVE ── 2/3 active ──────────────┐
│                                                                              │
│ ── WAN Core ─────────────────────────────────────────────────────────────── │
│  Device        Interface  Description       Status   In      ▂▃▅    Out    │
│  core-rtr-1    Gi0/0      To ISP-A          up       1.2G    ▃▅▇█   890M   │
│▸ core-rtr-2    Gi0/0      To DC-Fabric      up       2.1G    ▅▆▇█   1.8G   │
│                                                                              │
│ ── Branch Sites ─────────────────────────────────────────────────────────── │
│  branch-01     Eth1       WAN Uplink        up       45M     ▁▂▃▂   22M    │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│ ↻ 10s │ Last: 14:32:05 │ 7/7 OK                                             │
│ enter:detail  d:dashboards  i:identities  n:new  s:settings  ?:help  q:quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Columns**: Device, Interface, Description (ifAlias), Status (colored), In rate (auto-scaled), In sparkline (8-char), Out rate, Out sparkline, Util% (green <50%, yellow 50-80%, red >80%).

### Split-Screen Detail View

Activated by pressing Enter on a row. Shows 1-hour historical line charts for inbound and outbound traffic. Left/right arrows navigate between interfaces.

### Dashboard Switcher

Floating overlay (d key). Lists all dashboards with status (LIVE/STOPPED), interface count, poll interval. Start/stop/view/edit/delete operations.

### Identity Manager

Full-screen view (i key). Table of identities showing name, version, auth/priv protocols, usage count. Add/edit/test/delete operations.

### Dashboard Builder Wizard

Step-by-step flow (n key):
1. Name + default identity + poll interval
2. Add targets: host, label, identity, group → discover interfaces → checkbox picker
3. Review + save

### Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| j/k or ↑/↓ | All views | Navigate rows |
| Enter | Dashboard | Open detail view |
| Esc | Overlay/detail | Close, return to parent |
| d | Dashboard | Dashboard switcher |
| i | Dashboard | Identity manager |
| n | Dashboard | New dashboard wizard |
| s | Dashboard | Settings |
| r | Dashboard | Force refresh |
| ? | Anywhere | Help overlay |
| q / Ctrl+C | Anywhere | Quit |
| ←/→ | Detail view | Prev/next interface |

### Theme System

Base16 color system with 16 semantic slots per theme. 20+ built-in themes matching nbor's set. Color semantics:

- Base0B (green) = healthy/up
- Base08 (red) = down/error
- Base0A (yellow) = warning/degraded
- Base0D (blue) = headers/navigation
- Base03 (gray) = stale/disabled

Visual elements: Unicode box-drawing for borders, block elements (▁▂▃▄▅▆▇█) for sparklines, Braille patterns for high-res charts. Reverse-video selection, blinking LIVE indicator, bold for changed values, dim for stale.

## CLI Interface

```bash
# Monitoring
flo                                    # Launch TUI with dashboard switcher
flo --dashboard wan-links              # Launch into specific dashboard
flo --theme dracula                    # Override theme

# Identity management
flo identity add <name>                # Interactive credential prompts
flo identity list                      # Table of names + versions (no secrets)
flo identity test <name> --host <ip>   # Validate against live device
flo identity remove <name>             # Delete (warns if referenced)
flo identity export                    # Export names + versions as TOML

# Discovery
flo discover <host> --identity <name>  # Walk device, list interfaces
flo discover <host> --identity <name> --toml  # Output dashboard TOML snippet

# Config
flo config path                        # Print config directory
flo config theme <name>                # Set default theme
flo config default-identity <name>     # Set global default identity
```

## Data Model

```go
type Identity struct {
    Name      string
    Version   string        // "1", "2c", "3"
    Community string        // v1/v2c
    Username  string        // v3
    AuthProto string        // "MD5", "SHA", "SHA256", "SHA512"
    AuthPass  string
    PrivProto string        // "DES", "AES128", "AES192", "AES256"
    PrivPass  string
}

type Dashboard struct {
    Name            string
    DefaultIdentity string
    Interval        time.Duration
    MaxHistory      int
    Groups          []Group
}

type Group struct {
    Name    string
    Targets []Target
}

type Target struct {
    Host       string
    Label      string
    Identity   string    // references Identity.Name
    Port       int       // default 161
    Interfaces []string
}

type InterfaceStats struct {
    IfIndex     int
    Name        string
    Description string        // ifAlias
    Speed       uint64        // ifHighSpeed in Mbps
    Status      string        // "up", "down", "testing"
    InRate      float64       // current bps
    OutRate     float64       // current bps
    Utilization float64       // percentage
    History     *RingBuffer[RateSample]
    PollError   error
    LastPoll    time.Time
}

type RateSample struct {
    Timestamp time.Time
    InRate    float64       // bps
    OutRate   float64       // bps
}
```

## v1 Feature Set

- SNMP interface monitoring (v1/v2c/v3) with sparklines and utilization
- Encrypted identity management (CLI + TUI)
- Dashboard management (TOML configs + TUI builder wizard)
- Concurrent engine manager (background polling via goroutines)
- Device discovery (SNMP walk → interface picker)
- Split-screen detail view (1hr historical charts)
- Base16 theme system (20+ themes, persistent config)
- Counter wrap/reset detection
- Graceful degradation (per-target error handling)
- Cross-platform (macOS, Linux, Windows)

## v2+ Ideas

- Alert thresholds (per-interface utilization alerts, visual + terminal bell)
- Background daemon mode (keep polling after TUI exits)
- CSV/JSON export of historical data
- SNMP trap receiver
- Interface flap detection
- Comparison view (two interfaces side-by-side)
- Custom OID monitoring
- Config import from trafikwatch YAML
- Remote dashboard sharing

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — TUI styling
- `github.com/charmbracelet/bubbles` — Reusable TUI components
- `github.com/gosnmp/gosnmp` — SNMP v1/v2c/v3
- `github.com/BurntSushi/toml` — Config parsing
- `golang.org/x/crypto` — Argon2id key derivation, AES-GCM
- `golang.org/x/term` — Terminal size detection
