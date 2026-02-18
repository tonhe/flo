# flo

A beautiful TUI-based SNMP interface monitoring tool.

Built with [Go](https://go.dev), [Bubble Tea](https://github.com/charmbracelet/bubbletea), and the [Charm](https://charm.sh) ecosystem.

## Features

- **Real-time SNMP monitoring** with sparkline graphs and per-interface throughput
- **Concurrent multi-dashboard support** -- background polling continues when switching views
- **Identity picker modal** -- select from saved identities or create new ones inline, everywhere an identity is needed
- **Encrypted identity management** using AES-256-GCM with Argon2id key derivation
- **21 built-in Base16 themes** with live preview in the settings view
- **Dashboard builder wizard** -- create dashboards interactively from the TUI
- **Device discovery** via SNMP walks to enumerate interfaces before building dashboards
- **Split-screen detail view** with ASCII line charts for in/out traffic
- **SNMPv1, v2c, and v3 support** including AuthPriv (MD5, SHA, SHA-256, SHA-512 / DES, AES)
- **Cross-platform** -- Linux, macOS, and Windows
- **CLI commands** for scripting and automation alongside the TUI

## Installation

```
go install github.com/tonhe/flo@latest
```

Or build from source:

```
git clone https://github.com/tonhe/flo.git
cd flo
go build -o flo .
```

## Quick Start

### 1. Add an SNMP identity

```bash
flo identity add
```

You will be prompted for a name, SNMP version, and the relevant credentials (community string for v1/v2c, or username and auth/priv settings for v3).

### 2. Discover interfaces on a device

```bash
flo discover --identity myswitch 192.168.1.1
```

### 3. Create a dashboard

Place a TOML file in `~/.config/flo/dashboards/` (or use the built-in wizard with `n` inside the TUI):

```bash
flo   # launch the TUI, then press 'n' to open the dashboard builder
```

### 4. Launch and monitor

```bash
flo                          # interactive TUI
flo --dashboard mynetwork    # jump straight to a specific dashboard
flo --theme dracula          # override the theme for this session
```

## Usage

### TUI Mode

```
flo                           Launch the interactive monitor
flo --dashboard NAME          Start with a specific dashboard loaded
flo --theme NAME              Override the color theme for this session
```

### CLI Commands

```
flo identity list             List all saved identities
flo identity add              Add a new SNMP identity (interactive)
flo identity remove NAME      Remove an identity
flo identity test NAME HOST   Test SNMP connectivity to a device

flo discover --identity NAME HOST
                              Discover interfaces on a device via SNMP walk

flo config path               Show the config directory path
flo config theme NAME         Set the default theme
flo config identity NAME      Set the default identity

flo themes                    List all available themes
flo version                   Show version
flo help                      Show usage help
```

## Keyboard Shortcuts

### Global

| Key       | Action              |
|-----------|---------------------|
| `?`       | Toggle help overlay |
| `q`       | Quit                |
| `Ctrl+C`  | Quit                |

### Dashboard View

| Key       | Action                          |
|-----------|---------------------------------|
| `j` / `Down`  | Move cursor down           |
| `k` / `Up`    | Move cursor up             |
| `Enter`   | Open detail view for interface  |
| `d`       | Open dashboard switcher         |
| `n`       | Open dashboard builder wizard   |
| `i`       | Open identity manager           |
| `s`       | Open settings / theme picker    |
| `r`       | Force refresh                   |

### Detail View

| Key       | Action          |
|-----------|-----------------|
| `Esc`     | Back to dashboard |
| `h` / `Left`  | Previous interface |
| `l` / `Right` | Next interface     |

### Switcher / Lists

| Key       | Action              |
|-----------|---------------------|
| `j` / `Down`  | Move down       |
| `k` / `Up`    | Move up         |
| `Enter`   | Select / activate   |
| `Esc`     | Close               |
| `Tab`     | Next field          |

## Configuration

flo follows the XDG Base Directory Specification:

| Platform | Config directory                |
|----------|---------------------------------|
| Linux    | `~/.config/flo/`               |
| macOS    | `~/.config/flo/`               |
| Windows  | `%APPDATA%\flo\`               |

### Directory layout

```
~/.config/flo/
  config.toml           # Global settings (theme, default identity, poll interval)
  identities.enc        # Encrypted identity store (AES-256-GCM)
  dashboards/
    mynetwork.toml      # Dashboard definition
    datacenter.toml
```

### Global config (`config.toml`)

```toml
theme = "solarized-dark"
default_identity = "myswitch"
poll_interval = "10s"
max_history = 360
```

## Dashboard TOML Example

```toml
name = "mynetwork"
default_identity = "myswitch"
interval = "10s"
max_history = 360

[[groups]]
name = "Core Switches"

  [[groups.targets]]
  host = "192.168.1.1"
  label = "sw-core-01"
  interfaces = ["GigabitEthernet0/1", "GigabitEthernet0/2"]

  [[groups.targets]]
  host = "192.168.1.2"
  label = "sw-core-02"
  interfaces = ["GigabitEthernet0/1"]

[[groups]]
name = "Edge Routers"

  [[groups.targets]]
  host = "10.0.0.1"
  label = "rtr-edge-01"
  identity = "router-snmpv3"
  interfaces = ["GigabitEthernet0/0/0", "GigabitEthernet0/0/1"]
```

Targets inherit `default_identity` unless overridden with a per-target `identity` field. The default port is 161.

## Available Themes

flo ships with 21 Base16 themes. Set the default with `flo config theme NAME` or switch live in the TUI settings view (`s`).

| Theme | Theme | Theme |
|---|---|---|
| ayu-dark | catppuccin-latte | catppuccin-mocha |
| dracula | everforest | flo |
| github-dark | gruvbox-dark | gruvbox-light |
| horizon | kanagawa | monokai |
| nord | one-dark | palenight |
| rose-pine | solarized-dark | solarized-light |
| tokyo-night | tomorrow-night | zenburn |

## Architecture

flo is a single static binary with no external runtime dependencies.

- **TUI layer** -- [Bubble Tea](https://github.com/charmbracelet/bubbletea) drives the terminal UI with composable views (dashboard, detail, switcher, builder, settings, identity manager, help overlay). Themes use the Base16 color system via [Lip Gloss](https://github.com/charmbracelet/lipgloss).
- **Engine layer** -- A goroutine-based polling engine (`internal/engine`) manages concurrent SNMP sessions per dashboard. Each target runs its own poller goroutine that writes into lock-free ring buffers. The manager coordinates start/stop and provides snapshot reads to the TUI.
- **Identity layer** -- SNMP credentials are stored in an AES-256-GCM encrypted file with Argon2id key derivation from a master password (`internal/identity`). The `FLO_MASTER_KEY` environment variable can be used to skip the interactive prompt.
- **Dashboard layer** -- Dashboards are defined as TOML files (`internal/dashboard`). They can be created with the TUI wizard or written by hand.
- **CLI layer** -- Subcommands (`cmd/`) provide non-interactive access to identity management, device discovery, config, and theme listing for scripting workflows.

## Environment Variables

| Variable          | Description                                      |
|-------------------|--------------------------------------------------|
| `FLO_MASTER_KEY`  | Master password for the identity store (skips interactive prompt) |
| `XDG_CONFIG_HOME` | Override the config base directory (default: `~/.config`) |
| `XDG_DATA_HOME`   | Override the data base directory (default: `~/.local/share`) |

## License

TBD
