# TUI UX Polish — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix visual bugs, add quit confirmation, improve dashboard loading UX, and replace the builder edit mode with an nbor-style edit menu.

**Architecture:** Seven independent tasks ordered by complexity. Quick fixes first (version, status bars, quit, theme, loading delay), then the large edit menu rewrite. The new EditorView follows the same patterns as SettingsView: value-type receiver for Update/View, pointer for setters, (View, Cmd, Action) return pattern.

**Tech Stack:** Go, Bubble Tea (bubbletea/bubbles/lipgloss), TOML dashboards, gosnmp

---

## Task 1: Version + Build in Top Bar

**Files:**
- Create: `internal/version/version.go`
- Modify: `Makefile`
- Modify: `cmd/root.go`
- Modify: `tui/components/header.go`
- Modify: `tui/app.go`

**Step 1: Create version package**

Create `internal/version/version.go`:

```go
package version

// Version is the semantic version, injected via ldflags.
var Version = "dev"

// Build is the build timestamp (mmddyyyy.hhmm), injected via ldflags.
var Build = ""
```

**Step 2: Update Makefile**

Replace the top variables in `Makefile`:

```makefile
BINARY  := flo
GOFLAGS := -trimpath
VERSION := 0.1.0
BUILD   := $(shell date +%m%d%Y.%H%M)
LDFLAGS := -s -w -X github.com/tonhe/flo/internal/version.Version=$(VERSION) -X github.com/tonhe/flo/internal/version.Build=$(BUILD)
```

**Step 3: Update `cmd/root.go` line 39**

Add import `"github.com/tonhe/flo/internal/version"`. Replace the hardcoded version:

```go
case "version":
    fmt.Printf("flo v%s", version.Version)
    if version.Build != "" {
        fmt.Printf(" (%s)", version.Build)
    }
    fmt.Println()
```

**Step 4: Update header signature and add version segment**

In `tui/components/header.go`, change the function signature to accept `ver, build string` parameters. Add a version segment after the engines segment. (Full rewrite happens in Task 2.)

**Step 5: Update `tui/app.go` RenderHeader call**

Add import `"github.com/tonhe/flo/internal/version"`. Pass `version.Version` and `version.Build` to `RenderHeader`.

**Step 6: Build and verify**

Run: `make install && flo version`
Expected: `flo v0.1.0 (02182026.HHMM)`

**Step 7: Commit**

```
Add version package with ldflags injection and display in header
```

---

## Task 2: Status Bar Background Fix

**Files:**
- Modify: `tui/components/header.go`
- Modify: `tui/components/statusbar.go`

### Root Cause

Each `lipgloss.Render()` emits ANSI reset codes (`\e[0m`). The `" | "` separators between rendered segments have no background — they show the terminal default, creating visible gaps.

**Step 1: Rewrite `tui/components/header.go`**

Replace the function with a version where every character (separators, padding, filler) is rendered with explicit `.Background(bg)`:

```go
func RenderHeader(theme styles.Theme, dashName string, isLive bool, activeCount, totalCount, width int, ver, build string) string {
	bg := theme.Base01
	sep := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg).Render("  |  ")
	pad := lipgloss.NewStyle().Background(bg).Render(" ")

	left := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true).Render("flo")

	displayName := dashName
	if displayName == "" {
		displayName = "(no dashboard)"
	}
	center := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg).Render(displayName)

	status := "STOPPED"
	statusColor := theme.Base08
	if isLive {
		status = "LIVE"
		statusColor = theme.Base0B
	}
	right := lipgloss.NewStyle().Foreground(statusColor).Background(bg).Render(status)

	engines := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg).
		Render(fmt.Sprintf("%d/%d engines", activeCount, totalCount))

	versionStr := "v" + ver
	if build != "" {
		versionStr += "  " + build
	}
	versionSeg := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg).Render(versionStr)

	content := pad + left + sep + center + sep + right + sep + engines + sep + versionSeg

	contentWidth := lipgloss.Width(content)
	if contentWidth < width {
		filler := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-contentWidth))
		content += filler
	}
	return content
}
```

Add `"strings"` to imports.

**Step 2: Rewrite `tui/components/statusbar.go`**

Same approach: every segment and separator carries `.Background(bg)`. Also remove the stale `n:new` key binding (moved to switcher). Replace with `e:edit`:

```go
func RenderStatusBar(theme styles.Theme, interval time.Duration, lastPoll time.Time, okCount, totalCount, width int) string {
	bg := theme.Base01
	bgStyle := lipgloss.NewStyle().Background(bg)
	sep := bgStyle.Copy().Foreground(theme.Base03).Render(" | ")

	pollSeg := bgStyle.Copy().Foreground(theme.Base05).Render(fmt.Sprintf("poll: %s", interval))
	lastStr := "never"
	if !lastPoll.IsZero() {
		lastStr = lastPoll.Format("15:04:05")
	}
	lastSeg := bgStyle.Copy().Foreground(theme.Base05).Render(fmt.Sprintf("last: %s", lastStr))

	healthColor := theme.Base0B
	if okCount < totalCount {
		healthColor = theme.Base0A
	}
	healthSeg := lipgloss.NewStyle().Foreground(healthColor).Background(bg).Render(fmt.Sprintf("%d/%d OK", okCount, totalCount))

	topContent := bgStyle.Render(" ") + pollSeg + sep + lastSeg + sep + healthSeg
	topWidth := lipgloss.Width(topContent)
	if topWidth < width {
		topContent += bgStyle.Render(strings.Repeat(" ", width-topWidth))
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	spacer := bgStyle.Render("  ")

	keys := bgStyle.Render(" ") +
		keyStyle.Render("enter") + descStyle.Render(":detail") + spacer +
		keyStyle.Render("d") + descStyle.Render(":dashboards") + spacer +
		keyStyle.Render("i") + descStyle.Render(":identities") + spacer +
		keyStyle.Render("e") + descStyle.Render(":edit") + spacer +
		keyStyle.Render("s") + descStyle.Render(":settings") + spacer +
		keyStyle.Render("?") + descStyle.Render(":help") + spacer +
		keyStyle.Render("q") + descStyle.Render(":quit")

	keysWidth := lipgloss.Width(keys)
	if keysWidth < width {
		keys += bgStyle.Render(strings.Repeat(" ", width-keysWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, topContent, keys)
}
```

Add `"strings"` to imports.

**Step 3: Build and verify**

Run: `make install`
Verify: header and status bars have solid `Base01` background edge to edge.

**Step 4: Commit**

```
Fix status bar and header background gaps, remove stale n:new keybind
```

---

## Task 3: Quit from Detail View

**Files:**
- Modify: `tui/app.go`

**Step 1: Add `q` handler to StateDetail**

In `tui/app.go`, in the `case StateDetail:` block (line 237), add before `m.detail.Update(msg)`:

```go
case StateDetail:
    if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "q" {
        m.manager.StopAll()
        return m, tea.Quit
    }
    var cmd tea.Cmd
    var goBack bool
    m.detail, cmd, goBack = m.detail.Update(msg)
    // ... rest unchanged
```

**Step 2: Add `q` handler to StateSwitcher**

In the `case StateSwitcher:` block (line 257), add before `m.switcher.Update(msg)`:

```go
case StateSwitcher:
    if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "q" {
        m.manager.StopAll()
        return m, tea.Quit
    }
    // ... rest unchanged
```

**Step 3: Build and verify**

Run: `make install`
Verify: from detail view, press `q` — app quits. From switcher, press `q` — app quits.

**Step 4: Commit**

```
Add q-to-quit handler in detail view and switcher
```

---

## Task 4: Confirm-to-Quit Dialog

**Files:**
- Modify: `tui/app.go`

**Step 1: Add `confirmQuit` field to AppModel**

```go
type AppModel struct {
    // ... existing fields ...
    confirmQuit bool
}
```

**Step 2: Add `tryQuit` helper**

```go
func (m AppModel) tryQuit() (tea.Model, tea.Cmd) {
    if len(m.manager.ListEngines()) == 0 {
        return m, tea.Quit
    }
    m.confirmQuit = true
    return m, nil
}
```

**Step 3: Replace all quit sites with tryQuit**

Replace every `m.manager.StopAll(); return m, tea.Quit` with `return m.tryQuit()`:

- Global `ctrl+c` handler (line 163)
- StateDashboard `q` handler (line 180)
- StateDetail `q` handler (from Task 3)
- StateSwitcher `q` handler (from Task 3)

**Step 4: Add confirm dialog key handler**

After the help overlay check (line 154), before global key bindings:

```go
if m.confirmQuit {
    switch msg.String() {
    case "y":
        m.manager.StopAll()
        return m, tea.Quit
    case "n", "esc":
        m.confirmQuit = false
        return m, nil
    }
    return m, nil
}
```

**Step 5: Add renderQuitConfirm method**

```go
func (m AppModel) renderQuitConfirm() string {
    bodyHeight := m.height - 3
    if bodyHeight < 1 {
        bodyHeight = 1
    }
    sty := styles.NewStyles(m.theme)
    titleStyle := sty.ModalTitle
    textStyle := lipgloss.NewStyle().Foreground(m.theme.Base05)
    keyStyle := lipgloss.NewStyle().Foreground(m.theme.Base0D).Bold(true)
    dimStyle := lipgloss.NewStyle().Foreground(m.theme.Base04)

    content := lipgloss.JoinVertical(lipgloss.Left,
        titleStyle.Render("Quit?"),
        "",
        textStyle.Render("Engines are still running."),
        textStyle.Render("Quit anyway?"),
        "",
        dimStyle.Render(keyStyle.Render("[y]")+" yes    "+keyStyle.Render("[n]")+" no"),
    )

    modal := sty.ModalBorder.Width(36).Render(content)
    return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, modal)
}
```

**Step 6: Add to View()**

Before the help overlay check in View():

```go
if m.confirmQuit {
    modalBody := m.renderQuitConfirm()
    full := lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(modalBody), statusBar)
    return full
}
```

**Step 7: Build and verify**

Run: `make install`
Verify: with engines running, press `q` — dialog appears. Press `n` — dismissed. Press `q` then `y` — quits. With no engines, press `q` — quits immediately.

**Step 8: Commit**

```
Add confirm-to-quit dialog when engines are running
```

---

## Task 5: Theme on "No Dashboard" Screen

**Files:**
- Modify: `tui/app.go`

**Step 1: Apply bodyStyle to all View paths**

In `tui/app.go` View(), move the `bodyStyle` definition above the modal/confirm checks, and wrap ALL body content through `bodyStyle.Render()`:

```go
bodyHeight := m.height - 1 - 2
if bodyHeight < 1 {
    bodyHeight = 1
}
bodyStyle := lipgloss.NewStyle().
    Width(m.width).
    Height(bodyHeight).
    Background(m.theme.Base00).
    Foreground(m.theme.Base05)

if m.confirmQuit {
    modalBody := m.renderQuitConfirm()
    return lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(modalBody), statusBar)
}
if m.help.IsVisible() {
    modalBody := m.help.View()
    return lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(modalBody), statusBar)
}
if m.state == StateSwitcher {
    modalBody := m.switcher.View()
    return lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(modalBody), statusBar)
}

return lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(body), statusBar)
```

**Step 2: Build and verify**

Run: `make install`
Verify: "No dashboard loaded" screen has themed background. Help and switcher modals have themed background.

**Step 3: Commit**

```
Apply themed background to modal and empty-state screens
```

---

## Task 6: Dashboard Loading Delay

**Files:**
- Modify: `internal/engine/poller.go`
- Modify: `tui/views/dashboard.go`

**Step 1: Add `initTargetStats` method to poller**

In `internal/engine/poller.go`, add a new method that pre-populates empty stats (no SNMP calls):

```go
func (p *Poller) initTargetStats() {
    p.mu.Lock()
    defer p.mu.Unlock()
    for _, group := range p.dash.Groups {
        for _, target := range group.Targets {
            ts := &TargetStats{
                Host:  target.Host,
                Label: target.Label,
            }
            for _, ifName := range target.Interfaces {
                ts.Interfaces = append(ts.Interfaces, InterfaceStats{
                    Name:    ifName,
                    Status:  "",
                    History: NewRingBuffer[RateSample](p.dash.MaxHistory),
                })
            }
            p.data[target.Host] = ts
        }
    }
    p.notify()
}
```

**Step 2: Modify `Run()` to poll async**

Change `Run()` to call `initTargetStats()` first, then fire the first poll in a goroutine:

```go
func (p *Poller) Run() {
    p.initTargetStats()

    ticker := time.NewTicker(p.dash.Interval)
    defer ticker.Stop()

    go p.poll()

    for {
        select {
        case <-ticker.C:
            p.poll()
        case <-p.stopCh:
            p.cleanup()
            return
        }
    }
}
```

**Step 3: Handle pre-populated stats in `getOrCreateTargetStats`**

The existing method checks `if ts, ok := p.data[target.Host]; ok { return ts }` — but the pre-populated stats have `IfIndex == 0` (not resolved yet). Add interface resolution on first access:

```go
func (p *Poller) getOrCreateTargetStats(target dashboard.Target) *TargetStats {
    if ts, ok := p.data[target.Host]; ok {
        if len(ts.Interfaces) > 0 && ts.Interfaces[0].IfIndex == 0 {
            resolved := p.resolveInterfaces(target)
            for i, iface := range ts.Interfaces {
                if info, found := resolved[iface.Name]; found {
                    ts.Interfaces[i].IfIndex = info.IfIndex
                    ts.Interfaces[i].Speed = info.Speed
                    ts.Interfaces[i].Description = info.Description
                }
            }
        }
        return ts
    }
    // Fallback: should not happen after initTargetStats
    ts := &TargetStats{Host: target.Host, Label: target.Label}
    resolved := p.resolveInterfaces(target)
    for _, ifName := range target.Interfaces {
        stats := InterfaceStats{
            Name:    ifName,
            History: NewRingBuffer[RateSample](p.dash.MaxHistory),
        }
        if info, ok := resolved[ifName]; ok {
            stats.IfIndex = info.IfIndex
            stats.Speed = info.Speed
            stats.Description = info.Description
        }
        ts.Interfaces = append(ts.Interfaces, stats)
    }
    p.data[target.Host] = ts
    return ts
}
```

**Step 4: Show "..." in dashboard view for unpolled interfaces**

In `tui/views/dashboard.go`, in the status rendering section of `renderInterfaceRow`, add a case for empty status:

```go
case "":
    st := lipgloss.NewStyle().Foreground(v.theme.Base04)
    if selected {
        st = st.Background(v.theme.Base02)
    }
    statusStr = st.Render(padRight("...", wStatus))
```

Also show "---" for rates and utilization when `iface.Status == ""` (not yet polled).

**Step 5: Build and verify**

Run: `make install`
Verify: loading a dashboard shows interface rows immediately with "..." status and "---" for rates. After first poll (~10s), data populates.

**Step 6: Commit**

```
Show dashboard UI immediately with connecting status, poll async
```

---

## Task 7: Edit Dashboard Menu (nbor-style)

**Files:**
- Create: `tui/views/editor.go`
- Modify: `tui/app.go` (add StateEditor, wire editor)
- Modify: `tui/views/help.go` (add editor section)

This is the largest task. Creates a new `EditorView` that replaces the builder wizard's edit mode. The builder wizard remains for new dashboards only.

### EditorView Design

**Modes:**
- `modeMenu` — top-level with Settings section + Hosts section
- `modeInlineEdit` — editing a settings field inline
- `modeHostDetail` — viewing/editing a host's fields + interfaces
- `modeHostInline` — editing a host field inline
- `modeAddHost` — multi-step add host flow

**Navigation:** `> ` cursor indicator, up/down to navigate, Enter to edit/drill-in, `a` to add, `d` to delete, Esc auto-saves and closes.

### Step 1: Create `tui/views/editor.go`

Create the full EditorView with all types, constants, constructor, LoadDashboard, SetSize, Update (dispatches to mode-specific handlers), View (dispatches to mode-specific renderers), and save method.

**Key types:**

```go
type EditorAction int
const (
    EditorActionNone  EditorAction = iota
    EditorActionClose
    EditorActionSaved
)

type editorMode int
const (
    modeMenu       editorMode = iota
    modeInlineEdit
    modeHostDetail
    modeHostInline
    modeAddHost
)
```

**EditorView struct:**

```go
type EditorView struct {
    theme, sty, provider, width, height
    dashName, defaultIdentity, intervalStr string
    targets []dashboard.Target
    editPath string
    mode editorMode
    section editorSection  // sectionSettings or sectionHosts
    settingsCur, hostsCur, globalCur int
    detailHostIdx, detailCur int
    input textinput.Model
    editingWhat string
    identities []string
    SavedPath, err string
}
```

**Menu navigation:** globalCur traverses settings rows (0-2) then host rows (3+). `syncCursorFromGlobal()` maps to section + section-cursor. Enter on settings starts inline edit. Enter on host drills into host detail.

**Host detail:** 3 fixed fields (host, label, identity) + N interface rows. Enter for inline edit, `a` to add interface, `d` to delete.

**Add host:** Multi-step input (host → label → identity → interfaces), appends to targets list.

**Save:** Auto-save on Esc from top menu. Builds Dashboard struct, calls `dashboard.SaveDashboard()`.

**View rendering:** nbor-style with section headers in blue bold (Base0D), `> ` cursor in blue, active labels in bold, values in Base06, dimmed placeholders in Base03.

See the full code in the design section above (sections 7a–7l).

### Step 2: Add `StateEditor` to AppState enum

In `tui/app.go`:

```go
const (
    // ... existing states ...
    StateEditor
)
```

Add `editor views.EditorView` field to `AppModel`.

### Step 3: Replace `editDashboard()` to use EditorView

```go
func (m *AppModel) editDashboard(path string) {
    dash, err := dashboard.LoadDashboard(path)
    if err != nil { return }
    m.editor = views.NewEditorView(m.theme, m.provider)
    m.editor.LoadDashboard(dash, path)
    m.editor.SetSize(m.width, m.height-3)
    m.state = StateEditor
}
```

### Step 4: Handle StateEditor in Update

```go
case StateEditor:
    var cmd tea.Cmd
    var action views.EditorAction
    m.editor, cmd, action = m.editor.Update(msg)
    switch action {
    case views.EditorActionSaved:
        path := m.editor.SavedPath
        if dash, err := dashboard.LoadDashboard(path); err == nil {
            if m.activeDash != "" {
                _ = m.manager.Stop(m.activeDash)
            }
            if m.provider != nil {
                _ = m.manager.Start(dash, m.provider)
            }
            m.activeDash = dash.Name
        }
        m.state = StateDashboard
        return m, nil
    case views.EditorActionClose:
        m.state = StateDashboard
        return m, nil
    }
    return m, cmd
```

### Step 5: Handle StateEditor in View

```go
case StateEditor:
    body = m.editor.View()
```

### Step 6: Update help view

Add an "Edit Dashboard" section with: up/down navigate, enter edit/drill, a add, d delete, esc save & close.

### Step 7: Build and verify

Run: `make install`
- From dashboard, press `e` — editor opens with settings + hosts
- Navigate up/down, enter to inline-edit settings
- Enter on host → drill into host detail
- `a` to add host, `d` to delete
- Esc → auto-saves, returns to dashboard, engine restarts
- From switcher, `e` → same editor

### Step 8: Commit

```
Add nbor-style editor menu, replace builder edit mode
```

---

## Files Summary

| Task | Files Modified | Files Created |
|------|---------------|---------------|
| 1 | Makefile, cmd/root.go, header.go, app.go | internal/version/version.go |
| 2 | header.go, statusbar.go | — |
| 3 | app.go | — |
| 4 | app.go | — |
| 5 | app.go | — |
| 6 | poller.go, dashboard.go (view) | — |
| 7 | app.go, help.go | tui/views/editor.go |
