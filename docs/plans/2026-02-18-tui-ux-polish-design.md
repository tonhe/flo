# TUI UX Polish Design

Covers: edit menu redesign, status bar fixes, version/build display, quit improvements, theme consistency, and loading UX.

## 1. Edit Dashboard Menu (nbor-style)

Replace the builder wizard in edit mode with a nbor-style vertical menu. The builder wizard remains for new dashboards only.

### Entry Points

- `(e)` from dashboard view edits the active dashboard
- `(e)` from the switcher modal edits the selected dashboard
- Both paths open the same edit menu

### Edit Menu Layout

```
  Edit Dashboard: MIKAL

  --- Settings ---

  > Dashboard Name        MIKAL
    Default Identity      BeaconCiscoworks
    Poll Interval         10s

  --- Hosts ---

    bkh-core              Ethernet1/48
  > trh-core              TwentyFiveGigE3/1/6

  [a] add host  [enter] edit selected  [d] delete host  [esc] back
```

- Section headers: blue bold (Base0D)
- `> ` cursor indicator on focused row
- **Settings rows** (name, identity, interval): press Enter to inline-edit via textinput, Esc cancels, Enter commits
- **Hosts rows**: press Enter to drill into host detail, `a` to add, `d` to delete
- Changes auto-saved on Esc (leaving the menu)

### Host Detail Sub-View

```
  Edit Host: trh-core

  > Host                  10.30.5.2
    Label                 trh-core
    Identity              (default)

  --- Interfaces ---

  > TwentyFiveGigE3/1/6

  [a] add interface  [enter] edit  [d] delete  [esc] back
```

- Same nbor-style layout
- Enter on a field: inline-edit
- Enter on an interface: inline-edit the name
- Prepares for future per-interface settings (bandwidth, description)

## 2. Status Bar Background Fix

Each `lipgloss.Render()` emits ANSI reset codes that break the outer background between styled segments. Fix by rendering separators and padding as styled strings with explicit `.Background()` so every character has a background — no unstyled gaps.

Applies to both `header.go` and `statusbar.go`.

## 3. Version and Build Number in Top Bar

Add a `version` package with `Version` and `Build` variables. Inject via `-ldflags -X` in the Makefile. Build format: `mmddyyyy.hhmm`.

Header becomes:
```
 flo v0.1.0  |  MIKAL  |  LIVE  |  1/1 engines  |  02182026.1430
```

## 4. Quit from Detail View

Bug: `q` quit handler only exists in `StateDashboard`. The global `Quit` binding is `ctrl+c` only.

Fix: add `q` handler to `StateDetail` (and other non-text-input states like `StateSwitcher`). Cannot add to global handler since text-input views need to type `q`.

## 5. Confirm-to-Quit Dialog

When `q` or `ctrl+c` is pressed and engines are running, show a centered modal:

```
  +-----------------------------------+
  |  Engines are still running.       |
  |  Quit anyway?                     |
  |                                   |
  |  [y] yes    [n] no                |
  +-----------------------------------+
```

- `y`: stop engines, quit
- `n` or Esc: dismiss
- If no engines running: quit immediately, no dialog

## 6. Theme on "No Dashboard" Screen

Ensure the bodyStyle wrapper (with `Background(m.theme.Base00)`) is always applied, including when modals or empty states are rendered. Currently modal views bypass the bodyStyle.

## 7. Dashboard Loading Delay

First `poll()` in `Poller.Run()` does 3 synchronous SNMP BulkWalks per target before UI shows data. Fix: render the dashboard table immediately with a "Connecting..." or waiting indicator per interface, then let the first poll complete in the background. The tick loop already refreshes every second so data appears automatically once the poll finishes.
