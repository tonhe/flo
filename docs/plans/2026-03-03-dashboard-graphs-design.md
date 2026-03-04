# Dashboard Graph Panel + Time Scale

## Summary

Add a live graph panel to the bottom of the dashboard view that shows In/Out traffic charts for the currently highlighted interface. Charts update as the cursor moves through rows. Add time scale labels (X-axis) to charts with configurable format (relative, absolute, or both).

## Layout

The dashboard view splits vertically into two regions: table on top, graph panel on bottom.

```
┌─ Header ──────────────────────────────────────────────┐
│ flo | my-dashboard | LIVE | 2/2 engines | v0.3.0      │
├─ Table (dynamic height, min 5 rows + header) ─────────┤
│ Device    Interface    Status   In     Out   Util Trend│
│ router1   eth0         up     1.2G   850M   42%  ▁▃▅▇ │
│ router1   eth1         up     3.8G   2.1G   78%  ▅▆▇█ │◄── cursor
│ router2   ge-0/0/0     up     450M   200M   18%  ▁▂▃▂ │
├─ Separator (1 line, thin border, Base03) ─────────────┤
├─ Graph Panel (remaining height, min 8 rows) ──────────┤
│  In: 3.8G  peak: 5.2G  avg: 3.1G   │  Out: 2.1G ...  │
│  6G ░                               │  4G ░            │
│  4G ░░░▓▓▓▓▓▓▓▓█████▓▓▓░░░         │  2G ▓▓▓▓▓▓▓▓▓▓  │
│  2G ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓         │  0  ──────────  │
│  0  ─────────────────────────        │                  │
│     15m ago    10m ago    5m ago  now│  15m ago ... now │
├─ Status Bar ──────────────────────────────────────────┤
│ poll: 5s | last: 15:04:23 | 12/12 OK                  │
│ ↑/↓ navigate | d dashboards | enter detail | q quit   │
└────────────────────────────────────────────────────────┘
```

### Sizing Logic

- Total body height = terminal height - 3 (1 header + 2 status bar)
- Graph panel gets ~40% of body height, minimum 8 rows (title + 6 chart rows + time axis)
- Table gets the rest, minimum 6 rows (1 header + 5 data rows)
- Separator takes 1 row
- If terminal is too short for both minimums (< 15 body rows), table takes priority and graphs hide
- Two charts (In/Out) rendered side-by-side within the graph panel, separated by ` | ` (same pattern as existing detail view)

## Chart Enhancements

### Time Axis (X-axis)

A new bottom row on each chart showing time labels derived from `RateSample.Timestamp` values.

Relative mode (default):
```
     15m ago      10m ago      5m ago      now
```

Absolute mode:
```
     14:49       14:54       14:59    15:04
```

Both mode:
```
     15m ago      10m ago      5m ago    now 15:04
```

**Implementation details:**
- `RenderChart` accepts an optional `[]time.Time` parameter for timestamps corresponding to data points
- When timestamps are provided, the chart adds an X-axis label row
- Chart height calculation accounts for the extra row: title + chart rows + time axis = total height
- Labels are evenly spaced at roughly `chartWidth / 4` intervals to avoid overlap
- Time format controlled by a `TimeFormat` string parameter passed to the renderer

### Inline Stats in Title

Replace the current simple title (e.g. `"In Traffic"`) with stats:

```
In: 1.2G  peak: 3.8G  avg: 1.1G
```

- **Current rate**: most recent data point value, rendered in the bar color (green for In, cyan for Out)
- **Peak**: maximum value in the visible data slice, rendered in label color (dim)
- **Avg**: mean of the visible data slice, rendered in label color (dim)
- Stats are computed from the data slice passed to `RenderChart`, not stored separately

## Integration with DashboardView

### Data Flow

No new data fetching needed. The existing pipeline provides everything:

1. `DashboardView.SelectedInterface()` returns `*InterfaceStats` for the cursor row
2. `InterfaceStats.History` is a `*RingBuffer[RateSample]` with timestamps
3. `RateSample` has `Timestamp`, `InRate`, `OutRate`
4. On each `TickMsg` (1s), the snapshot refreshes and graphs re-render

### View Changes

- `DashboardView` gains a `graphPanelHeight` field, calculated in `SetSize()`
- `DashboardView.View()` renders: table section + separator + graph panel
- Table's `visibleHeight` is reduced by `graphPanelHeight + 1` (separator)
- Graph panel calls `RenderChart` with In/Out data + timestamps from history
- Chart colors match existing detail view: green (Base0B) for In, cyan (Base0C) for Out

### Detail View

Unchanged. Pressing Enter still opens the full detail view with info panel + charts. The dashboard graph panel is the quick preview; the detail view is the deep dive.

The detail view's charts will also benefit from the time axis and inline stats enhancements since they use the same `RenderChart` function.

## Settings

### New Config Field

```go
TimeFormat string `toml:"time_format"` // "relative", "absolute", "both"
```

- Default: `"relative"`
- Added to `Config` struct in `internal/config/config.go`
- Added to `Dashboard` struct in `internal/dashboard/dashboard.go` (per-dashboard override, falls back to global config)

### Settings View

New row in the settings form for time format. Cycle through three options on Enter (same UX pattern as sort mode toggle in interface picker): relative → absolute → both → relative.

### Data Flow

Config → AppModel → DashboardView → RenderChart. The time format string is passed through to the chart renderer which formats labels accordingly.

## Files to Modify

| File | Change |
|------|--------|
| `tui/components/chart.go` | Add time axis rendering, inline stats title, accept timestamps parameter |
| `tui/views/dashboard.go` | Split layout: table + separator + graph panel, pass data to charts |
| `tui/views/detail.go` | Pass timestamps to RenderChart (benefits from shared enhancement) |
| `internal/config/config.go` | Add `TimeFormat` field with default |
| `internal/dashboard/dashboard.go` | Add `TimeFormat` field (per-dashboard override) |
| `tui/views/settings.go` | Add time format toggle row |
| `tui/app.go` | Pass time format config through to dashboard view |
