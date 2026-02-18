# Identity Picker Design

## Summary

Replace all free-text identity input fields with a reusable modal overlay picker component. The picker shows existing identities, a "none" option to clear, and supports inline creation of new identities.

## Component: `IdentityPickerModel`

New file: `tui/components/identitypicker.go`

Self-contained Bubble Tea model with two modes:

### modeList

Scrollable list with cursor navigation:

1. `(none)` — clears the identity field (returns empty string)
2. Existing identities from `Provider.List()` — displayed as `Name  v2c` or `Name  v3 user:bob SHA/AES128`
3. `+ New Identity` — transitions to modeForm

Keys: `up`/`down` navigate, `enter` selects, `esc` cancels.

### modeForm

Full identity creation form matching the existing field pattern in `identity.go`:

- Name, Version (cycle), Community (v1/v2c), Username (v3), AuthProto (cycle), AuthPass, PrivProto (cycle), PrivPass
- Dynamic field visibility based on SNMP version
- `tab`/`shift+tab` navigate fields, `enter` advances or saves on last field, `esc` returns to list

On save: calls `Provider.Add()`, refreshes the list, auto-selects the new identity.

### Return signature

```go
func (p IdentityPickerModel) Update(msg tea.Msg) (IdentityPickerModel, tea.Cmd, PickerAction)

const (
    PickerNone      PickerAction = iota // no action, keep showing
    PickerSelected                       // user picked; call SelectedName()
    PickerCancelled                      // user pressed esc
)

func (p IdentityPickerModel) SelectedName() string
```

### Rendering

Centered modal overlay using `lipgloss.Place()`, matching `SwitcherView` pattern. Box contains title ("Select Identity" or "New Identity"), list/form content, and a help line.

## Integration Points

Four views get picker integration, all following the same pattern:

| View | Fields | Trigger |
|------|--------|---------|
| `builder.go` | Step 1 dashboard default, Step 2 per-target identity | `enter` on identity field |
| `editor.go` | Dashboard default identity, per-host identity | `enter` on identity field in inline edit |
| `settings.go` | Global default identity | `enter` on identity field |

Each view adds:

```go
showPicker bool
picker     components.IdentityPickerModel
```

In `Update()`: when `showPicker` is true, route all input to the picker. In `View()`: when `showPicker` is true, render the picker as an overlay.

## Data Flow

```
User focuses identity field -> presses enter
  -> View sets showPicker = true, initializes picker with Provider
  -> Picker calls Provider.List() -> renders list
  -> User navigates:
      -> Selects identity -> PickerSelected, view reads SelectedName()
      -> Selects "(none)" -> PickerSelected, SelectedName() = ""
      -> Selects "+ New Identity" -> picker enters modeForm
          -> Fills form -> Provider.Add() -> auto-selects new name -> PickerSelected
      -> Presses esc -> PickerCancelled, field unchanged
```

## Identity Resolution Hierarchy (unchanged)

1. `Target.Identity` (per-host override)
2. `Dashboard.DefaultIdentity` (dashboard default)
3. `Config.DefaultIdentity` (global default)

## Edge Cases

- **Provider nil**: Picker shows empty list with only "(none)" and a message "No identity store. Press i to set up." Does not handle store creation inline.
- **Duplicate name on create**: `Provider.Add()` returns an error; picker displays the error inline and keeps the form open.
- **Empty identity list**: Only "(none)" and "+ New Identity" entries shown.

## Design Decisions

- **No search/filter**: Identity lists are small (<20 typical). Simple scrollable list is sufficient.
- **Own form implementation**: The picker has its own creation form rather than extracting from `identity.go`, avoiding a refactor of the existing identity manager.
- **Modal overlay**: Consistent with the `SwitcherView` pattern. User stays in context of the current view.
