# TUI UX Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate TUI navigation, add identity store bootstrap, and enable dashboard editing.

**Architecture:** Three independent features sharing the same view layer. Task 1 restructures navigation by moving "new" into the switcher. Task 2 adds a setup mode to the identity view. Task 3 extends the builder view with edit-mode support.

**Tech Stack:** Go, Bubble Tea, lipgloss, charmbracelet/bubbles textinput

---

### Task 1: Move "New Dashboard" into the Switcher

**Files:**
- Modify: `tui/keys/keys.go`
- Modify: `tui/views/switcher.go`
- Modify: `tui/app.go`
- Modify: `tui/views/help.go`

**Step 1: Add `ActionNew` and `ActionEdit` to switcher actions**

In `tui/views/switcher.go`, add two new actions to the enum (lines 20-29):

```go
const (
	ActionNone SwitcherAction = iota
	ActionClose
	ActionSwitch
	ActionStop
	ActionNew
	ActionEdit
)
```

**Step 2: Handle `(n)` and `(e)` keys in the switcher**

In `tui/views/switcher.go`, add two new cases in `Update()` after the `"x"` handler (line 139):

```go
		case msg.String() == "n":
			return v, nil, ActionNew

		case msg.String() == "e":
			if len(v.items) > 0 {
				return v, nil, ActionEdit
			}
			return v, nil, ActionNone
```

**Step 3: Update the switcher footer hints**

In `tui/views/switcher.go`, update the help line (lines 179-184) to:

```go
	help := fmt.Sprintf(
		"%s:switch  %s:new  %s:edit  %s:stop  %s:close",
		helpKeyStyle.Render("enter"),
		helpKeyStyle.Render("n"),
		helpKeyStyle.Render("e"),
		helpKeyStyle.Render("x"),
		helpKeyStyle.Render("esc"),
	)
```

**Step 4: Remove `New` from top-level KeyMap**

In `tui/keys/keys.go`, remove the `New` field from the `KeyMap` struct (line 14) and from `DefaultKeyMap` (line 32). Add `Edit` in its place:

```go
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	Quit      key.Binding
	Dashboard key.Binding
	Identity  key.Binding
	Edit      key.Binding
	Settings  key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Left      key.Binding
	Right     key.Binding
	Tab       key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:        key.NewBinding(key.WithKeys("up"), key.WithHelp("up", "up")),
	Down:      key.NewBinding(key.WithKeys("down"), key.WithHelp("down", "down")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Dashboard: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dashboards")),
	Identity:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identities")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Settings:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Left:      key.NewBinding(key.WithKeys("left"), key.WithHelp("left", "left")),
	Right:     key.NewBinding(key.WithKeys("right"), key.WithHelp("right", "right")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
}
```

**Step 5: Update app.go to route switcher actions**

In `tui/app.go`:

a) Remove the `(n)` handler from `StateDashboard` (lines 192-198). Add `(e)` handler for editing active dashboard:

```go
			// Open the dashboard editor on 'e'
			if key.Matches(msg, keys.DefaultKeyMap.Edit) {
				if m.activeDash != "" {
					m.editActiveDashboard()
				}
				return m, nil
			}
```

b) In the `StateSwitcher` action handling (after `ActionStop`, line 274), add:

```go
			case views.ActionNew:
				m.builder = views.NewBuilderView(m.theme, m.provider)
				m.builder.SetSize(m.width, m.height-3)
				m.state = StateBuilder
				return m, nil

			case views.ActionEdit:
				if item := m.switcher.SelectedItem(); item != nil {
					m.editDashboard(item.FilePath)
				}
				return m, nil
```

c) Add the helper methods at the bottom of app.go:

```go
// editDashboard loads a dashboard TOML and opens it in the builder for editing.
func (m *AppModel) editDashboard(path string) {
	dash, err := dashboard.LoadDashboard(path)
	if err != nil {
		return
	}
	m.builder = views.NewBuilderView(m.theme, m.provider)
	m.builder.LoadDashboard(dash, path)
	m.builder.SetSize(m.width, m.height-3)
	m.state = StateBuilder
}

// editActiveDashboard opens the currently active dashboard for editing.
func (m *AppModel) editActiveDashboard() {
	dashDir, err := config.GetDashboardsDir()
	if err != nil {
		return
	}
	names, err := dashboard.ListDashboards(dashDir)
	if err != nil {
		return
	}
	for _, name := range names {
		path := filepath.Join(dashDir, name+".toml")
		dash, loadErr := dashboard.LoadDashboard(path)
		if loadErr == nil && dash.Name == m.activeDash {
			m.editDashboard(path)
			return
		}
	}
}
```

d) Update the `BuilderActionSave` handler (lines 288-298) to stop the old engine before restarting:

```go
			case views.BuilderActionSave:
				path := m.builder.SavedPath
				if dash, err := dashboard.LoadDashboard(path); err == nil {
					// Stop old engine if editing an active dashboard
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
```

**Step 6: Update help overlay**

In `tui/views/help.go`, replace the Dashboard section (lines 87-96) with:

```go
	// Dashboard section
	lines = append(lines, sectionStyle.Render("Dashboard"))
	lines = append(lines, bindingLine("q", "Quit"))
	lines = append(lines, bindingLine("Up / Down", "Navigate interfaces"))
	lines = append(lines, bindingLine("Enter", "Detail view"))
	lines = append(lines, bindingLine("d", "Dashboard switcher"))
	lines = append(lines, bindingLine("e", "Edit active dashboard"))
	lines = append(lines, bindingLine("i", "Identity manager"))
	lines = append(lines, bindingLine("s", "Settings"))
	lines = append(lines, bindingLine("r", "Force refresh"))
	lines = append(lines, "")

	// Switcher section
	lines = append(lines, sectionStyle.Render("Dashboard Switcher"))
	lines = append(lines, bindingLine("Enter", "Switch to dashboard"))
	lines = append(lines, bindingLine("n", "New dashboard"))
	lines = append(lines, bindingLine("e", "Edit dashboard"))
	lines = append(lines, bindingLine("x", "Stop engine"))
	lines = append(lines, bindingLine("Esc", "Close"))
	lines = append(lines, "")
```

And remove the old "Switcher / Identity / Builder" section (lines 103-107). Replace with just:

```go
	// Identity / Builder section
	lines = append(lines, sectionStyle.Render("Identity / Builder"))
	lines = append(lines, bindingLine("Esc", "Close / Back"))
	lines = append(lines, bindingLine("Enter", "Select / Confirm"))
	lines = append(lines, "")
```

**Step 7: Build and verify**

Run: `make install`
Expected: Clean build, binary copied to ~/bin/

**Step 8: Commit**

```bash
git add tui/keys/keys.go tui/views/switcher.go tui/app.go tui/views/help.go
git commit -F <(printf 'Move new/edit dashboard into switcher modal\n\nRemove top-level (n) keybinding. Add (n) new and (e) edit inside\nthe dashboard switcher. Add (e) from dashboard view for active\ndashboard editing.')
```

---

### Task 2: Identity Store Bootstrap from TUI

**Files:**
- Modify: `tui/views/identity.go`
- Modify: `tui/app.go`
- Modify: `main.go`

**Step 1: Add `IdentitySetup` mode and setup fields to the identity view**

In `tui/views/identity.go`, add the new mode (line 20-25):

```go
const (
	IdentityList  IdentityMode = iota
	IdentityForm
	IdentitySetup
)
```

Add setup fields to the `IdentityView` struct (after line 69):

```go
	// Setup state (first-time store creation)
	setupPass    textinput.Model
	setupConfirm textinput.Model
	setupFocus   int
	storePath    string
	OnStoreCreated func(identity.Provider) // callback to update app provider
```

**Step 2: Add `SetStorePath` method and setup initialization**

Add after the `SetProvider` method (line 85):

```go
// SetStorePath sets the path for creating a new identity store.
func (v *IdentityView) SetStorePath(path string) {
	v.storePath = path
}

// SetOnStoreCreated sets the callback invoked after a new store is created.
func (v *IdentityView) SetOnStoreCreated(fn func(identity.Provider)) {
	v.OnStoreCreated = fn
}

// initSetup prepares the setup mode for first-time store creation.
func (v *IdentityView) initSetup() {
	v.mode = IdentitySetup
	v.err = ""
	v.setupFocus = 0

	v.setupPass = textinput.New()
	v.setupPass.Placeholder = "master password (blank for none)"
	v.setupPass.CharLimit = 128
	v.setupPass.Width = 40
	v.setupPass.EchoMode = textinput.EchoPassword
	v.setupPass.Focus()

	v.setupConfirm = textinput.New()
	v.setupConfirm.Placeholder = "confirm password"
	v.setupConfirm.CharLimit = 128
	v.setupConfirm.Width = 40
	v.setupConfirm.EchoMode = textinput.EchoPassword
}
```

**Step 3: Update `Refresh` to trigger setup mode when no provider**

Replace the `Refresh` method (lines 94-117):

```go
func (v *IdentityView) Refresh() {
	v.err = ""
	if v.provider == nil {
		if v.storePath != "" {
			v.initSetup()
		} else {
			v.summaries = nil
		}
		return
	}
	sums, err := v.provider.List()
	if err != nil {
		v.err = fmt.Sprintf("Failed to load identities: %v", err)
		v.summaries = nil
		return
	}
	sort.Slice(sums, func(i, j int) bool {
		return sums[i].Name < sums[j].Name
	})
	v.summaries = sums
	if v.cursor >= len(v.summaries) {
		v.cursor = len(v.summaries) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}
```

**Step 4: Add setup Update and View handlers**

Add the `IdentitySetup` case to `Update` (lines 121-129):

```go
func (v IdentityView) Update(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	switch v.mode {
	case IdentityList:
		return v.updateList(msg)
	case IdentityForm:
		return v.updateForm(msg)
	case IdentitySetup:
		return v.updateSetup(msg)
	}
	return v, nil, false
}
```

Add the `IdentitySetup` case to `View` (lines 132-139):

```go
func (v IdentityView) View() string {
	switch v.mode {
	case IdentityForm:
		return v.viewForm()
	case IdentitySetup:
		return v.viewSetup()
	default:
		return v.viewList()
	}
}
```

Add the setup update handler:

```go
func (v IdentityView) updateSetup(msg tea.Msg) (IdentityView, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Escape):
			return v, nil, true

		case msg.String() == "tab", msg.String() == "shift+tab":
			if v.setupFocus == 0 {
				v.setupFocus = 1
				v.setupPass.Blur()
				v.setupConfirm.Focus()
			} else {
				v.setupFocus = 0
				v.setupConfirm.Blur()
				v.setupPass.Focus()
			}
			return v, nil, false

		case msg.String() == "enter":
			if v.setupFocus == 0 {
				v.setupFocus = 1
				v.setupPass.Blur()
				v.setupConfirm.Focus()
				return v, nil, false
			}
			// Validate passwords match
			pass := v.setupPass.Value()
			confirm := v.setupConfirm.Value()
			if pass != confirm {
				v.err = "Passwords don't match"
				return v, nil, false
			}
			// Create the store
			store, err := identity.NewFileStore(v.storePath, []byte(pass))
			if err != nil {
				v.err = fmt.Sprintf("Failed to create store: %v", err)
				return v, nil, false
			}
			v.provider = store
			if v.OnStoreCreated != nil {
				v.OnStoreCreated(store)
			}
			v.mode = IdentityList
			v.err = ""
			v.Refresh()
			return v, nil, false

		default:
			var cmd tea.Cmd
			if v.setupFocus == 0 {
				v.setupPass, cmd = v.setupPass.Update(msg)
			} else {
				v.setupConfirm, cmd = v.setupConfirm.Update(msg)
			}
			return v, cmd, false
		}
	}
	return v, nil, false
}
```

Add the setup view renderer:

```go
func (v IdentityView) viewSetup() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)
	labelStyle := v.sty.FormLabel
	activeLabelStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	helpStyle := lipgloss.NewStyle().Foreground(v.theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + titleStyle.Render("Identity Store Setup") + "\n")
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render("No identity store found. Create one to store SNMP credentials.") + "\n")
	b.WriteString("  " + dimStyle.Render("Leave password blank for a no-password vault.") + "\n")
	b.WriteString("\n")

	if v.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(v.theme.Base08)
		b.WriteString("  " + errStyle.Render(v.err) + "\n\n")
	}

	type field struct {
		label string
		view  string
	}
	fields := []field{
		{"Password", v.setupPass.View()},
		{"Confirm", v.setupConfirm.View()},
	}

	for i, f := range fields {
		isFocused := i == v.setupFocus
		indicator := "  "
		lbl := labelStyle
		if isFocused {
			indicatorStyle := lipgloss.NewStyle().Foreground(v.theme.Base0D).Bold(true)
			indicator = indicatorStyle.Render("> ")
			lbl = activeLabelStyle
		}
		b.WriteString(fmt.Sprintf("  %s%s%s\n", indicator, lbl.Render(padRight(f.label+":", 18)), f.view))
	}

	b.WriteString("\n")
	b.WriteString("  " + helpStyle.Render(fmt.Sprintf(
		"%s navigate  %s create  %s cancel",
		keyStyle.Render("[tab]"),
		keyStyle.Render("[enter]"),
		keyStyle.Render("[esc]"),
	)) + "\n")

	return b.String()
}
```

**Step 5: Update main.go to pass store path even when no store exists**

In `main.go`, the current code (lines 82-110) skips the provider entirely when the store file doesn't exist. Update to always pass `storePath`:

Replace lines 79-110 with:

```go
	// Open identity store if it exists.
	var provider identity.Provider
	storePath, err := config.GetIdentityStorePath()
	if err != nil {
		storePath = ""
	}
	if storePath != "" {
		if _, statErr := os.Stat(storePath); statErr == nil {
			// Try empty password first
			store, storeErr := identity.NewFileStore(storePath, []byte(""))
			if storeErr == nil {
				provider = store
			} else {
				// Empty password didn't work, try env var or prompt
				password := []byte(os.Getenv("FLO_MASTER_KEY"))
				if len(password) == 0 {
					fmt.Fprint(os.Stderr, "Master password: ")
					password, err = term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Fprintln(os.Stderr)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
						os.Exit(1)
					}
				}
				store, storeErr = identity.NewFileStore(storePath, password)
				if storeErr != nil {
					fmt.Fprintf(os.Stderr, "Error opening identity store: %v\n", storeErr)
					os.Exit(1)
				}
				provider = store
			}
		}
	}
```

No changes to the logic itself — the only difference is that `storePath` is always available for the app model.

**Step 6: Update `NewAppModel` to pass store path and wire up the callback**

In `tui/app.go`, add `storePath string` field to `AppModel` (after line 53):

```go
	storePath     string
```

Update `NewAppModel` signature and body (lines 59-77) to accept and store it:

```go
func NewAppModel(cfg *config.Config, mgr *engine.Manager, provider identity.Provider, startDash string, storePath string) AppModel {
```

And in the return block, add:

```go
		storePath:     storePath,
```

Update the identity view initialization in `NewAppModel`:

```go
		identity:      views.NewIdentityView(theme, provider),
```

Then set the store path after creation. Actually, it's simpler to add it to the `(i)` key handler. Update the identity open handler (lines 186-190):

```go
			if key.Matches(msg, keys.DefaultKeyMap.Identity) {
				m.identity.SetSize(m.width, m.height-3)
				m.identity.SetStorePath(m.storePath)
				m.identity.SetOnStoreCreated(func(p identity.Provider) {
					m.provider = p
				})
				m.identity.Refresh()
				m.state = StateIdentity
				return m, nil
			}
```

Also update the settings theme handler that recreates identity view (line 320) to preserve provider:

```go
				m.identity = views.NewIdentityView(m.theme, m.provider)
```

(This is already correct — just noting it doesn't need changing.)

**Step 7: Update `main.go` call to `NewAppModel`**

In `main.go` line 113, pass the store path:

```go
	model := tui.NewAppModel(cfg, mgr, provider, dashboardFlag, storePath)
```

**Step 8: Build and verify**

Run: `make install`
Expected: Clean build, binary copied to ~/bin/

**Step 9: Commit**

```bash
git add tui/views/identity.go tui/app.go main.go
git commit -F <(printf 'Add identity store bootstrap from TUI\n\nWhen no identity store exists, pressing (i) shows a setup form\nto create one with an optional master password.')
```

---

### Task 3: Dashboard Edit Mode in Builder

**Files:**
- Modify: `tui/views/builder.go`

**Step 1: Add edit mode fields to BuilderView**

In `tui/views/builder.go`, add fields to the struct (after line 69):

```go
	editMode bool   // true when editing an existing dashboard
	editPath string // file path to overwrite in edit mode
```

**Step 2: Add `LoadDashboard` method**

Add after `SetSize` (line 149):

```go
// LoadDashboard populates the builder from an existing dashboard for editing.
func (b *BuilderView) LoadDashboard(dash *dashboard.Dashboard, path string) {
	b.editMode = true
	b.editPath = path

	// Step 1 fields
	b.nameInput.SetValue(dash.Name)
	b.identityInput.SetValue(dash.DefaultIdentity)
	b.intervalInput.SetValue(dash.Interval.String())

	// Step 2: flatten all groups' targets
	b.targets = nil
	for _, g := range dash.Groups {
		for _, t := range g.Targets {
			b.targets = append(b.targets, t)
		}
	}

	// If there are targets, start at target list (not adding mode)
	if len(b.targets) > 0 {
		b.addingTarget = false
		b.targetCursor = 0
	}
}
```

**Step 3: Update `saveDashboard` to handle edit mode**

In `tui/views/builder.go`, update `saveDashboard()` (lines 721-786). Replace the path calculation block (lines 765-777):

```go
	var path string
	if b.editMode {
		path = b.editPath
	} else {
		dashDir, err := config.GetDashboardsDir()
		if err != nil {
			b.err = fmt.Sprintf("Failed to get dashboards dir: %v", err)
			return b, nil, BuilderActionNone
		}

		slug := slugify(name)
		path = filepath.Join(dashDir, slug+".toml")

		if err := config.EnsureDirs(); err != nil {
			b.err = fmt.Sprintf("Failed to create directories: %v", err)
			return b, nil, BuilderActionNone
		}
	}
```

**Step 4: Update view titles to show "Edit" vs "New"**

In `viewStepName()` (line 270), replace the title rendering:

```go
	title := "New Dashboard"
	if b.editMode {
		title = "Edit Dashboard"
	}
	s.WriteString("  " + titleStyle.Render(title) + "  " + stepStyle.Render("Step 1 of 3") + "\n")
```

In `viewStepTargets()` (line 560), same change:

```go
	title := "New Dashboard"
	if b.editMode {
		title = "Edit Dashboard"
	}
	s.WriteString("  " + titleStyle.Render(title) + "  " + stepStyle.Render("Step 2 of 3 - Add Targets") + "\n")
```

In `viewStepReview()` (line 804), same change:

```go
	title := "New Dashboard"
	if b.editMode {
		title = "Edit Dashboard"
	}
	s.WriteString("  " + titleStyle.Render(title) + "  " + stepStyle.Render("Step 3 of 3 - Review") + "\n")
```

Also update the "Will save to" hint (line 850) to show the actual path in edit mode:

```go
	if b.editMode {
		s.WriteString("  " + saveHint.Render(fmt.Sprintf("Will save to: %s", filepath.Base(b.editPath))) + "\n")
	} else {
		slug := slugify(name)
		s.WriteString("  " + saveHint.Render(fmt.Sprintf("Will save to: %s.toml", slug)) + "\n")
	}
```

**Step 5: Build and verify**

Run: `make install`
Expected: Clean build, binary copied to ~/bin/

**Step 6: Commit**

```bash
git add tui/views/builder.go
git commit -F <(printf 'Add edit mode to dashboard builder\n\nLoadDashboard() pre-populates the wizard from existing config.\nEdit mode overwrites the original file instead of creating new.')
```

---

### Task 4: Final build, verify, and update ROADMAP

**Step 1: Full build**

Run: `make install`
Expected: Clean build

**Step 2: Update ROADMAP.md**

Move the three completed items out of ROADMAP.md. Create CHANGELOG.md with:

```markdown
# Changelog

## Unreleased

- Move "New Dashboard" under Dashboards as a sub-option of the switcher modal
- Add identity store creation from the TUI (setup form with optional master password)
- Add/remove devices and interfaces on an existing dashboard (edit mode in builder)
```

**Step 3: Commit**

```bash
git add ROADMAP.md CHANGELOG.md
git commit -F <(printf 'Update roadmap and changelog for TUI improvements')
```
