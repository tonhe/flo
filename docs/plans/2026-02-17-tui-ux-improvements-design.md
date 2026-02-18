# TUI UX Improvements Design

Covers the first three roadmap items: consolidating navigation, identity store bootstrap, and dashboard editing.

## 1. Move "New Dashboard" Under Dashboards

Remove `(n)` as a top-level keybinding. Instead, pressing `(n)` inside the dashboard switcher modal launches the full-screen builder wizard.

### Changes

**keys.go** - Remove `New` from `KeyMap`.

**switcher.go** - Add `ActionNew` to `SwitcherAction`. Handle `(n)` in `Update()` returning `ActionNew`. Update footer hints to show `[n] new`.

**app.go** - Remove the `(n)` handler from `StateDashboard`. Handle `ActionNew` from the switcher by transitioning to `StateBuilder`.

**help.go** - Move `(n)` from the "Dashboard" section to a "Dashboard Switcher" section.

## 2. Identity Store Bootstrap from TUI

When no identity store exists and the user presses `(i)`, show a setup form to create one with an optional master password.

### Changes

**identity.go** - Add `IdentitySetup` mode. When the view opens and the store doesn't exist, show a form with password + confirm fields (blank = no password). On submit, create the store via `identity.NewFileStore()` and transition to `IdentityList`.

**app.go** - Handle the case where no provider exists at startup. Pass the store path to the identity view so it can bootstrap. After creation, update `m.provider` for the rest of the app.

## 3. Edit Existing Dashboards

Reuse the builder wizard in "edit mode" to modify existing dashboards. Accessible via `(e)` from both the switcher modal and the active dashboard view.

### Changes

**builder.go** - Add `LoadDashboard(dash, path)` method to pre-populate all fields from an existing dashboard. Add `editMode bool` field. In edit mode, `saveDashboard()` overwrites the original file instead of creating a new one.

**switcher.go** - Add `ActionEdit` to `SwitcherAction`. Handle `(e)` returning `ActionEdit` with the selected dashboard path. Add `(e) edit` to footer hints.

**keys.go** - Add `Edit` key binding (`e`).

**app.go** - Handle `ActionEdit` from switcher: load TOML, create builder with `LoadDashboard()`, transition to `StateBuilder`. Handle `(e)` in `StateDashboard` for editing the active dashboard. After save in edit mode: stop old engine, reload config, start new engine.

**help.go** - Add `(e) edit` to both dashboard view and switcher sections.
