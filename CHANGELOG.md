# Changelog

## Unreleased

- Fix status bar background bleed-through by propagating Background to inner styles
- Fix modal rendering by replacing ANSI-unaware overlayCenter with lipgloss.Place()
- Fix modal title injection that corrupted ANSI border lines
- Add solid background to modal dialogs
- Move "New Dashboard" under Dashboards as a sub-option of the switcher modal
- Add identity store creation from the TUI (setup form with optional master password)
- Add/remove devices and interfaces on an existing dashboard (edit mode in builder)
- Add version and build number to header bar (injected via ldflags)
- Fix status bar and header background gaps (render all segments with explicit background)
- Add quit from detail view and switcher (q key)
- Add confirm-to-quit dialog when engines are running
- Apply themed background to modal and empty-state screens
- Show dashboard UI immediately with "..." connecting status, first poll runs async
- Replace builder edit mode with nbor-style editor menu (inline editing, host drill-in)
