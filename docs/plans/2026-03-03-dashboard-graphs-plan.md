# Dashboard Graph Panel + Time Scale — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a live graph panel to the bottom of the dashboard view showing In/Out traffic charts for the highlighted interface, with time-axis labels and inline stats.

**Architecture:** Enhance `RenderChart` to accept timestamps and a time format, adding an X-axis row and stats-enriched title. Split the dashboard view vertically: table on top, separator, graph panel on bottom. Add `TimeFormat` config field with settings UI toggle.

**Tech Stack:** Go, Bubble Tea (bubbletea), Lip Gloss (lipgloss), TOML config

**Design doc:** `docs/plans/2026-03-03-dashboard-graphs-design.md`

---

### Task 1: Add `TimeFormat` to Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestDefaultConfigTimeFormat(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TimeFormat != "relative" {
		t.Errorf("expected default time format 'relative', got %q", cfg.TimeFormat)
	}
}

func TestConfigSaveLoadTimeFormat(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")

	cfg := DefaultConfig()
	cfg.TimeFormat = "absolute"

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if loaded.TimeFormat != "absolute" {
		t.Errorf("expected time format 'absolute', got %q", loaded.TimeFormat)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestDefaultConfigTimeFormat -v && go test ./internal/config/ -run TestConfigSaveLoadTimeFormat -v`
Expected: FAIL — `TimeFormat` field doesn't exist yet.

**Step 3: Implement — add `TimeFormat` to Config**

In `internal/config/config.go`, add the field to the `Config` struct:

```go
type Config struct {
	Theme           string        `toml:"theme"`
	DefaultIdentity string        `toml:"default_identity"`
	PollInterval    time.Duration `toml:"-"`
	PollIntervalStr string        `toml:"poll_interval"`
	MaxHistory      int           `toml:"max_history"`
	TimeFormat      string        `toml:"time_format"`
}
```

Set the default in `DefaultConfig()`:

```go
func DefaultConfig() *Config {
	return &Config{
		Theme:           "solarized-dark",
		DefaultIdentity: "",
		PollInterval:    10 * time.Second,
		PollIntervalStr: "10s",
		MaxHistory:      360,
		TimeFormat:      "relative",
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS.

**Step 5: Build**

Run: `make install`
Expected: Clean build.

**Step 6: Commit**

```
feat: add TimeFormat config field
```

---

### Task 2: Enhance `RenderChart` with Time Axis and Inline Stats

This is the core chart enhancement. The existing `RenderChart` function signature changes to accept optional timestamps and time format. To avoid breaking existing callers, introduce a new `ChartOptions` struct.

**Files:**
- Modify: `tui/components/chart.go`
- Create: `tui/components/chart_test.go`

**Step 1: Write the failing tests**

Create `tui/components/chart_test.go`:

```go
package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func testColors() ChartColors {
	return ChartColors{
		BarFg:   lipgloss.Color("#00ff00"),
		LabelFg: lipgloss.Color("#888888"),
		TitleFg: lipgloss.Color("#0000ff"),
		Bg:      lipgloss.Color("#000000"),
	}
}

func TestRenderChartBasic(t *testing.T) {
	data := []float64{100, 200, 300, 400, 500}
	result := RenderChart(data, 30, 8, "Test", testColors())
	lines := strings.Split(result, "\n")
	if len(lines) != 8 {
		t.Errorf("expected 8 lines, got %d", len(lines))
	}
}

func TestRenderChartEmpty(t *testing.T) {
	result := RenderChart(nil, 30, 8, "Test", testColors())
	if result == "" {
		t.Error("expected non-empty output for nil data")
	}
}

func TestRenderChartWithOptions(t *testing.T) {
	now := time.Now()
	data := []float64{100, 200, 300, 400, 500}
	timestamps := make([]time.Time, len(data))
	for i := range timestamps {
		timestamps[i] = now.Add(time.Duration(i-len(data)+1) * 30 * time.Second)
	}

	opts := ChartOptions{
		Timestamps: timestamps,
		TimeFormat: "relative",
	}
	// height=10: 1 title + 7 chart rows + 1 time axis + 1 for stats title = needs at least 10
	result := RenderChartWithOptions(data, 40, 10, testColors(), opts)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
}

func TestRenderChartStatsTitle(t *testing.T) {
	data := []float64{1_000_000, 2_000_000, 3_000_000}
	opts := ChartOptions{
		Label: "In",
	}
	result := RenderChartWithOptions(data, 50, 8, testColors(), opts)
	// Title should contain the label, current, peak, avg
	if !strings.Contains(result, "In") {
		t.Error("expected title to contain label 'In'")
	}
}

func TestFormatTimeLabel(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		ts       time.Time
		format   string
		expected string
	}{
		{"relative_now", now, "relative", "now"},
		{"relative_5m", now.Add(-5 * time.Minute), "relative", "5m ago"},
		{"relative_1h", now.Add(-1 * time.Hour), "relative", "1h ago"},
		{"absolute", now, "absolute", now.Format("15:04")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimeLabel(tt.ts, now, tt.format)
			if got != tt.expected {
				t.Errorf("FormatTimeLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./tui/components/ -run TestRenderChart -v && go test ./tui/components/ -run TestFormatTimeLabel -v`
Expected: FAIL — `ChartOptions`, `RenderChartWithOptions`, `FormatTimeLabel` don't exist.

**Step 3: Implement `ChartOptions`, `FormatTimeLabel`, and `RenderChartWithOptions`**

Add to `tui/components/chart.go`:

```go
import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ChartOptions holds optional configuration for enhanced chart rendering.
type ChartOptions struct {
	Timestamps []time.Time // timestamps corresponding to data points (for X-axis)
	TimeFormat string      // "relative", "absolute", or "both"
	Label      string      // short label like "In" or "Out" (used in stats title)
}

// FormatTimeLabel formats a timestamp as a time label for the X-axis.
func FormatTimeLabel(ts, now time.Time, format string) string {
	switch format {
	case "absolute":
		return ts.Format("15:04")
	case "both":
		ago := now.Sub(ts)
		if ago < 30*time.Second {
			return "now " + now.Format("15:04")
		}
		return formatRelative(ago)
	default: // "relative"
		ago := now.Sub(ts)
		if ago < 30*time.Second {
			return "now"
		}
		return formatRelative(ago)
	}
}

// formatRelative formats a duration as a short relative label.
func formatRelative(d time.Duration) string {
	switch {
	case d >= time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	case d >= time.Minute:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	default:
		s := int(d.Seconds())
		return fmt.Sprintf("%ds ago", s)
	}
}

// RenderChartWithOptions renders an ASCII chart with optional time axis and stats title.
// data: values to plot (oldest to newest)
// width/height: total dimensions including all elements
// colors: themed colors
// opts: optional timestamps, time format, label
func RenderChartWithOptions(data []float64, width, height int, colors ChartColors, opts ChartOptions) string {
	if width < 10 {
		width = 10
	}
	if height < 4 {
		height = 4
	}

	barStyle := lipgloss.NewStyle().Foreground(colors.BarFg).Background(colors.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(colors.LabelFg).Background(colors.Bg)
	titleStyle := lipgloss.NewStyle().Foreground(colors.TitleFg).Background(colors.Bg)
	barTitleStyle := lipgloss.NewStyle().Foreground(colors.BarFg).Background(colors.Bg)
	emptyStyle := lipgloss.NewStyle().Background(colors.Bg)

	labelWidth := 8
	chartWidth := width - labelWidth
	if chartWidth < 2 {
		chartWidth = 2
	}

	// Calculate vertical budget
	hasTimeAxis := len(opts.Timestamps) > 0 && opts.TimeFormat != ""
	titleRows := 1
	timeAxisRows := 0
	if hasTimeAxis {
		timeAxisRows = 1
	}
	chartHeight := height - titleRows - timeAxisRows
	if chartHeight < 2 {
		chartHeight = 2
	}

	var lines []string

	// Trim data and timestamps to chart width
	if len(data) > chartWidth {
		data = data[len(data)-chartWidth:]
	}
	timestamps := opts.Timestamps
	if len(timestamps) > chartWidth {
		timestamps = timestamps[len(timestamps)-chartWidth:]
	}

	// Build stats title
	title := opts.Label
	if len(data) > 0 {
		current := data[len(data)-1]
		peak := data[0]
		sum := 0.0
		for _, v := range data {
			sum += v
			if v > peak {
				peak = v
			}
		}
		avg := sum / float64(len(data))

		currentStr := barTitleStyle.Render(fmt.Sprintf("%s: %s", opts.Label, FormatRate(current)))
		peakStr := labelStyle.Render(fmt.Sprintf("peak: %s", FormatRate(peak)))
		avgStr := labelStyle.Render(fmt.Sprintf("avg: %s", FormatRate(avg)))
		title = fmt.Sprintf("%s  %s  %s", currentStr, peakStr, avgStr)
	}
	if title == "" {
		title = " "
	}
	// Pad title to full width
	titleLen := lipgloss.Width(title)
	if titleLen < width {
		title = "  " + title + emptyStyle.Render(strings.Repeat(" ", width-titleLen-2))
	}
	lines = append(lines, title)

	// Handle empty data
	if len(data) == 0 {
		for i := 0; i < chartHeight+timeAxisRows; i++ {
			lines = append(lines, emptyStyle.Render(strings.Repeat(" ", width)))
		}
		return strings.Join(lines, "\n")
	}

	// Find min/max for scaling
	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == minVal {
		maxVal = minVal + 1
	}
	if minVal > 0 {
		minVal = 0
	}
	spread := maxVal - minVal

	// Chart grid (same logic as RenderChart)
	for row := chartHeight - 1; row >= 0; row-- {
		rowTopVal := minVal + spread*float64(row+1)/float64(chartHeight)
		label := fmt.Sprintf("%7s ", FormatRate(rowTopVal))
		if len(label) > labelWidth {
			label = label[len(label)-labelWidth:]
		}

		var barChars strings.Builder
		var emptyChars strings.Builder

		padding := chartWidth - len(data)
		for p := 0; p < padding; p++ {
			emptyChars.WriteRune(' ')
		}

		for _, v := range data {
			cellBottom := minVal + spread*float64(row)/float64(chartHeight)
			cellTop := minVal + spread*float64(row+1)/float64(chartHeight)
			cellRange := cellTop - cellBottom

			if v <= cellBottom {
				barChars.WriteRune(' ')
			} else if v >= cellTop {
				barChars.WriteRune(chartBlocks[8])
			} else {
				fraction := (v - cellBottom) / cellRange
				idx := int(math.Round(fraction * 8))
				if idx < 0 {
					idx = 0
				}
				if idx > 8 {
					idx = 8
				}
				barChars.WriteRune(chartBlocks[idx])
			}
		}

		line := labelStyle.Render(label) +
			emptyStyle.Render(emptyChars.String()) +
			barStyle.Render(barChars.String())
		lines = append(lines, line)
	}

	// Time axis row
	if hasTimeAxis && len(timestamps) > 0 {
		now := time.Now()
		timeAxisLine := renderTimeAxis(timestamps, chartWidth, labelWidth, now, opts.TimeFormat, labelStyle, emptyStyle)
		lines = append(lines, timeAxisLine)
	}

	return strings.Join(lines, "\n")
}

// renderTimeAxis builds the X-axis time label row.
// Places ~4 evenly-spaced labels across the chart width.
func renderTimeAxis(timestamps []time.Time, chartWidth, labelWidth int, now time.Time, format string, labelStyle, emptyStyle lipgloss.Style) string {
	// Build a character buffer for the time axis
	axis := make([]byte, chartWidth)
	for i := range axis {
		axis[i] = ' '
	}

	// Calculate label positions (4 labels: start, 1/3, 2/3, end)
	numLabels := 4
	if chartWidth < 30 {
		numLabels = 2
	} else if chartWidth < 50 {
		numLabels = 3
	}

	padding := chartWidth - len(timestamps)
	if padding < 0 {
		padding = 0
	}

	for i := 0; i < numLabels; i++ {
		// Position in the data array
		var dataIdx int
		if numLabels == 1 {
			dataIdx = len(timestamps) - 1
		} else {
			dataIdx = i * (len(timestamps) - 1) / (numLabels - 1)
		}
		if dataIdx >= len(timestamps) {
			dataIdx = len(timestamps) - 1
		}

		label := FormatTimeLabel(timestamps[dataIdx], now, format)

		// Position on the axis (accounting for left padding from empty data)
		pos := padding + dataIdx
		// Center the label on the position
		start := pos - len(label)/2
		if start < 0 {
			start = 0
		}
		if start+len(label) > chartWidth {
			start = chartWidth - len(label)
		}
		if start < 0 {
			continue
		}

		// Place label characters
		for j, ch := range []byte(label) {
			if start+j < chartWidth {
				axis[start+j] = ch
			}
		}
	}

	// Build the line: label-width padding + axis
	pad := strings.Repeat(" ", labelWidth)
	return labelStyle.Render(pad) + emptyStyle.Render(string(axis))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tui/components/ -v`
Expected: All PASS.

**Step 5: Build**

Run: `make install`
Expected: Clean build.

**Step 6: Commit**

```
feat: add RenderChartWithOptions with time axis and inline stats
```

---

### Task 3: Update Detail View to Use `RenderChartWithOptions`

The detail view should pass timestamps through to the enhanced chart renderer. This validates the new API works with an existing caller before tackling the dashboard integration.

**Files:**
- Modify: `tui/views/detail.go`

**Step 1: Update `extractRateData` to also return timestamps**

In `tui/views/detail.go`, change the return signature and extract timestamps:

```go
// extractRateData pulls InRate, OutRate, and Timestamp slices from the interface history.
func (v DetailView) extractRateData() (inData, outData []float64, timestamps []time.Time) {
	if v.ifaceStats == nil || v.ifaceStats.History == nil {
		return nil, nil, nil
	}

	samples := v.ifaceStats.History.All()
	if len(samples) == 0 {
		return nil, nil, nil
	}

	inData = make([]float64, len(samples))
	outData = make([]float64, len(samples))
	timestamps = make([]time.Time, len(samples))
	for i, s := range samples {
		inData[i] = s.InRate
		outData[i] = s.OutRate
		timestamps[i] = s.Timestamp
	}
	return inData, outData, timestamps
}
```

**Step 2: Update `renderDetail` to use `RenderChartWithOptions`**

In the `renderDetail` method, change the chart rendering calls:

Replace the existing chart rendering block (the `inData, outData := v.extractRateData()` section and the two `RenderChart` calls) with:

```go
	inData, outData, timestamps := v.extractRateData()

	inColors := components.ChartColors{
		BarFg:   v.theme.Base0B,
		LabelFg: v.theme.Base04,
		TitleFg: v.theme.Base0D,
		Bg:      v.theme.Base00,
	}
	outColors := components.ChartColors{
		BarFg:   v.theme.Base0C,
		LabelFg: v.theme.Base04,
		TitleFg: v.theme.Base0D,
		Bg:      v.theme.Base00,
	}

	inOpts := components.ChartOptions{
		Timestamps: timestamps,
		TimeFormat: v.timeFormat,
		Label:      "In",
	}
	outOpts := components.ChartOptions{
		Timestamps: timestamps,
		TimeFormat: v.timeFormat,
		Label:      "Out",
	}

	inChart := components.RenderChartWithOptions(inData, chartWidth, chartHeight, inColors, inOpts)
	outChart := components.RenderChartWithOptions(outData, chartWidth, chartHeight, outColors, outOpts)
```

**Step 3: Add `timeFormat` field to `DetailView` and a setter**

Add the field to the struct and a setter method:

```go
type DetailView struct {
	theme       styles.Theme
	sty         *styles.Styles
	targetLabel string
	ifaceStats  *engine.InterfaceStats
	width       int
	height      int
	timeFormat  string
}

func (v *DetailView) SetTimeFormat(format string) {
	v.timeFormat = format
}
```

Default to `"relative"` — set in `NewDetailView`:

```go
func NewDetailView(theme styles.Theme) DetailView {
	return DetailView{
		theme:      theme,
		sty:        styles.NewStyles(theme),
		timeFormat: "relative",
	}
}
```

**Step 4: Build**

Run: `make install`
Expected: Clean build.

**Step 5: Commit**

```
feat: use RenderChartWithOptions in detail view for time axis and stats
```

---

### Task 4: Add Graph Panel to Dashboard View

This is the main feature: splitting the dashboard view into table + graph panel.

**Files:**
- Modify: `tui/views/dashboard.go`

**Step 1: Add graph panel fields to `DashboardView`**

Add fields to the struct:

```go
type DashboardView struct {
	theme      styles.Theme
	sty        *styles.Styles
	snapshot   *engine.DashboardSnapshot
	cursor     int
	width      int
	height     int
	totalRows  int
	offset     int
	timeFormat string
}
```

Add a setter:

```go
func (v *DashboardView) SetTimeFormat(format string) {
	v.timeFormat = format
}
```

Default in `NewDashboardView`:

```go
func NewDashboardView(theme styles.Theme) DashboardView {
	return DashboardView{
		theme:      theme,
		sty:        styles.NewStyles(theme),
		timeFormat: "relative",
	}
}
```

**Step 2: Calculate layout split in `SetSize`**

No changes needed to `SetSize` signature — the split is computed at render time in `View()` based on `v.height`.

**Step 3: Update `View()` to render table + separator + graph panel**

Replace the existing `View()` method:

```go
const (
	minTableRows = 6  // 1 header + 5 data rows
	minGraphRows = 8  // 1 title + 6 chart + 1 time axis
)

func (v DashboardView) View() string {
	if v.snapshot == nil || len(v.snapshot.Groups) == 0 {
		return v.renderEmpty()
	}

	// Calculate layout split
	graphHeight := 0
	separatorHeight := 0
	tableHeight := v.height

	// Only show graphs if we have enough room for both table and graphs
	if v.height >= minTableRows+minGraphRows+1 {
		// Give ~40% to graphs, rest to table
		graphHeight = v.height * 40 / 100
		if graphHeight < minGraphRows {
			graphHeight = minGraphRows
		}
		separatorHeight = 1
		tableHeight = v.height - graphHeight - separatorHeight
		if tableHeight < minTableRows {
			tableHeight = minTableRows
			graphHeight = v.height - tableHeight - separatorHeight
		}
	}

	// Render table portion
	tableContent := v.renderTableWithHeight(tableHeight)

	if graphHeight == 0 {
		return tableContent
	}

	// Render separator
	sepStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base03).
		Background(v.theme.Base00)
	separator := sepStyle.Render(strings.Repeat("─", v.width))

	// Render graph panel
	graphPanel := v.renderGraphPanel(graphHeight)

	return lipgloss.JoinVertical(lipgloss.Left, tableContent, separator, graphPanel)
}
```

**Step 4: Extract table rendering into `renderTableWithHeight`**

Rename and modify `renderTable` to accept an explicit height:

```go
func (v DashboardView) renderTableWithHeight(tableHeight int) string {
	wDevice, wIface, wStatus, wIn, wOut, wUtil, wSpark := v.columnWidths()

	var lines []string

	headerStyle := v.sty.TableHeader
	header := fmt.Sprintf(
		"%s%s%s%s%s%s%s",
		headerStyle.Render(padRight("Device", wDevice)),
		headerStyle.Render(padRight("Interface", wIface)),
		headerStyle.Render(padRight("Status", wStatus)),
		headerStyle.Render(padLeft("In", wIn)),
		headerStyle.Render(padLeft("Out", wOut)),
		headerStyle.Render(padLeft("Util", wUtil)),
		headerStyle.Render(padRight("Trend", wSpark)),
	)
	lines = append(lines, header)

	type row struct {
		isGroup bool
		text    string
	}
	var rows []row

	rowIdx := 0
	for _, g := range v.snapshot.Groups {
		groupLine := v.sty.GroupHeader.Render(
			padRight(fmt.Sprintf("--- %s ---", g.Name), v.width),
		)
		rows = append(rows, row{isGroup: true, text: groupLine})

		for _, t := range g.Targets {
			for _, iface := range t.Interfaces {
				rowText := v.renderInterfaceRow(
					t.Label, iface,
					wDevice, wIface, wStatus, wIn, wOut, wUtil, wSpark,
					rowIdx == v.cursor,
				)
				rows = append(rows, row{isGroup: false, text: rowText})
				rowIdx++
			}
		}
	}

	// Apply vertical scrolling using the provided tableHeight
	visibleHeight := tableHeight - 1 // subtract header
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	cursorCombinedIdx := 0
	ifaceIdx := 0
	for i, r := range rows {
		if !r.isGroup {
			if ifaceIdx == v.cursor {
				cursorCombinedIdx = i
				break
			}
			ifaceIdx++
		}
	}

	startIdx := 0
	if cursorCombinedIdx >= visibleHeight {
		startIdx = cursorCombinedIdx - visibleHeight + 1
	}
	endIdx := startIdx + visibleHeight
	if endIdx > len(rows) {
		endIdx = len(rows)
		startIdx = endIdx - visibleHeight
		if startIdx < 0 {
			startIdx = 0
		}
	}

	for i := startIdx; i < endIdx; i++ {
		lines = append(lines, rows[i].text)
	}

	return strings.Join(lines, "\n")
}
```

Remove the old `renderTable` method (it is fully replaced by `renderTableWithHeight`).

**Step 5: Implement `renderGraphPanel`**

Add this method to `DashboardView`:

```go
func (v DashboardView) renderGraphPanel(panelHeight int) string {
	_, iface := v.SelectedInterface()
	if iface == nil || iface.History == nil {
		emptyStyle := lipgloss.NewStyle().
			Foreground(v.theme.Base04).
			Background(v.theme.Base00)
		msg := emptyStyle.Render("No interface data")
		return lipgloss.Place(v.width, panelHeight, lipgloss.Center, lipgloss.Center, msg,
			lipgloss.WithWhitespaceBackground(v.theme.Base00))
	}

	// Extract data from history
	samples := iface.History.All()
	if len(samples) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(v.theme.Base04).
			Background(v.theme.Base00)
		msg := emptyStyle.Render("Waiting for data...")
		return lipgloss.Place(v.width, panelHeight, lipgloss.Center, lipgloss.Center, msg,
			lipgloss.WithWhitespaceBackground(v.theme.Base00))
	}

	inData := make([]float64, len(samples))
	outData := make([]float64, len(samples))
	timestamps := make([]time.Time, len(samples))
	for i, s := range samples {
		inData[i] = s.InRate
		outData[i] = s.OutRate
		timestamps[i] = s.Timestamp
	}

	// Chart dimensions: two charts side by side with separator
	chartWidth := (v.width - 3) / 2
	if chartWidth < 15 {
		chartWidth = 15
	}
	chartHeight := panelHeight

	inColors := components.ChartColors{
		BarFg:   v.theme.Base0B,
		LabelFg: v.theme.Base04,
		TitleFg: v.theme.Base0D,
		Bg:      v.theme.Base00,
	}
	outColors := components.ChartColors{
		BarFg:   v.theme.Base0C,
		LabelFg: v.theme.Base04,
		TitleFg: v.theme.Base0D,
		Bg:      v.theme.Base00,
	}

	inOpts := components.ChartOptions{
		Timestamps: timestamps,
		TimeFormat: v.timeFormat,
		Label:      "In",
	}
	outOpts := components.ChartOptions{
		Timestamps: timestamps,
		TimeFormat: v.timeFormat,
		Label:      "Out",
	}

	inChart := components.RenderChartWithOptions(inData, chartWidth, chartHeight, inColors, inOpts)
	outChart := components.RenderChartWithOptions(outData, chartWidth, chartHeight, outColors, outOpts)

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(v.theme.Base03).Background(v.theme.Base00)
	sepLines := make([]string, chartHeight)
	for i := range sepLines {
		sepLines[i] = sepStyle.Render(" | ")
	}
	sep := strings.Join(sepLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, inChart, sep, outChart)
}
```

**Step 6: Add `time` import**

Add `"time"` to the imports in `dashboard.go` (needed for `time.Time` in the extracted timestamps).

**Step 7: Update `ensureVisible` to use table height**

The `ensureVisible` method currently uses `v.height` for the visible window. It should account for the graph panel. Update it:

```go
func (v *DashboardView) ensureVisible() {
	// Calculate table-only height (same logic as View)
	tableHeight := v.height
	if v.height >= minTableRows+minGraphRows+1 {
		graphHeight := v.height * 40 / 100
		if graphHeight < minGraphRows {
			graphHeight = minGraphRows
		}
		tableHeight = v.height - graphHeight - 1
		if tableHeight < minTableRows {
			tableHeight = minTableRows
		}
	}

	visible := tableHeight - 1 // subtract header
	if visible < 1 {
		visible = 1
	}
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+visible {
		v.offset = v.cursor - visible + 1
	}
}
```

**Step 8: Build**

Run: `make install`
Expected: Clean build.

**Step 9: Commit**

```
feat: add graph panel to dashboard view with split layout
```

---

### Task 5: Wire `TimeFormat` Through App Model

Connect the config's `TimeFormat` to both dashboard and detail views.

**Files:**
- Modify: `tui/app.go`

**Step 1: Pass `TimeFormat` in `NewAppModel`**

After creating `dashboard` and `detail` in `NewAppModel`, set their time formats:

In the `NewAppModel` function, after the return struct is built, the dashboard and detail already get created. Add `SetTimeFormat` calls. Since the struct is returned directly, modify the construction:

```go
func NewAppModel(cfg *config.Config, mgr *engine.Manager, provider identity.Provider, startDash string, storePath string) AppModel {
	theme := styles.DefaultTheme
	if t := styles.GetThemeByName(cfg.Theme); t != nil {
		theme = *t
	}
	dashView := views.NewDashboardView(theme)
	dashView.SetTimeFormat(cfg.TimeFormat)
	detailView := views.NewDetailView(theme)
	detailView.SetTimeFormat(cfg.TimeFormat)
	return AppModel{
		state:         StateDashboard,
		theme:         theme,
		config:        cfg,
		manager:       mgr,
		provider:      provider,
		dashboard:     dashView,
		switcher:      views.NewSwitcherView(theme),
		detail:        detailView,
		identity:      views.NewIdentityView(theme, provider),
		startDashName: startDash,
		storePath:     storePath,
	}
}
```

**Step 2: Propagate after theme change**

In the `SettingsSaved` handler in `Update`, after recreating views with the new theme, also set the time format. The config object is already updated by the settings save. Find the `SettingsSaved` case and add `SetTimeFormat` calls after each view is created:

```go
case views.SettingsSaved:
	if t := styles.GetThemeByName(m.settings.SavedTheme); t != nil {
		m.theme = *t
		bodyHeight := m.height - 3
		m.dashboard = views.NewDashboardView(m.theme)
		m.dashboard.SetSize(m.width, bodyHeight)
		m.dashboard.SetTimeFormat(m.config.TimeFormat)
		m.switcher = views.NewSwitcherView(m.theme)
		m.switcher.SetSize(m.width, bodyHeight)
		m.detail = views.NewDetailView(m.theme)
		m.detail.SetSize(m.width, bodyHeight)
		m.detail.SetTimeFormat(m.config.TimeFormat)
		m.identity = views.NewIdentityView(m.theme, m.provider)
		m.identity.SetSize(m.width, bodyHeight)
	}
	m.state = StateDashboard
	return m, nil
```

**Step 3: Build**

Run: `make install`
Expected: Clean build.

**Step 4: Commit**

```
feat: wire TimeFormat config through app model to views
```

---

### Task 6: Add Time Format Toggle to Settings View

**Files:**
- Modify: `tui/views/settings.go`

**Step 1: Add the time format field**

Update the field constants:

```go
const (
	settingsFieldTheme      = 0
	settingsFieldIdentity   = 1
	settingsFieldInterval   = 2
	settingsFieldHistory    = 3
	settingsFieldTimeFormat = 4
	settingsFieldCount      = 5
)
```

Add a field to the `SettingsView` struct:

```go
type SettingsView struct {
	// ... existing fields ...
	timeFormatIndex int // 0=relative, 1=absolute, 2=both
}
```

**Step 2: Initialize in `NewSettingsView`**

Add after the existing input setup:

```go
	timeFormatIdx := 0
	switch cfg.TimeFormat {
	case "absolute":
		timeFormatIdx = 1
	case "both":
		timeFormatIdx = 2
	}
```

And add `timeFormatIndex: timeFormatIdx` to the return struct.

**Step 3: Add helper for time format display**

```go
var timeFormats = []string{"relative", "absolute", "both"}

func (s SettingsView) selectedTimeFormat() string {
	if s.timeFormatIndex >= 0 && s.timeFormatIndex < len(timeFormats) {
		return timeFormats[s.timeFormatIndex]
	}
	return "relative"
}
```

**Step 4: Update `hasChanges`**

Add a check:

```go
func (s SettingsView) hasChanges() bool {
	// ... existing checks ...
	if s.selectedTimeFormat() != s.config.TimeFormat {
		return true
	}
	return false
}
```

**Step 5: Handle left/right cycling for time format field**

In the `Update` method, in the `Left` and `Right` key handlers, add cases for the time format field (same pattern as theme cycling):

In the `Left` handler, after the theme block:

```go
if s.cursor == settingsFieldTimeFormat {
	s.timeFormatIndex--
	if s.timeFormatIndex < 0 {
		s.timeFormatIndex = len(timeFormats) - 1
	}
	s.changed = true
	return s, nil, SettingsNone
}
```

In the `Right` handler, after the theme block:

```go
if s.cursor == settingsFieldTimeFormat {
	s.timeFormatIndex++
	if s.timeFormatIndex >= len(timeFormats) {
		s.timeFormatIndex = 0
	}
	s.changed = true
	return s, nil, SettingsNone
}
```

**Step 6: Add to `save` method**

Before the existing `config.SaveConfig` call, add:

```go
s.config.TimeFormat = s.selectedTimeFormat()
```

**Step 7: Add row to `View` rendering**

Add a new row to the `rows` slice in the `View()` method:

```go
	timeFormatDisplay := fmt.Sprintf("< %s >  (%d/%d)", s.selectedTimeFormat(), s.timeFormatIndex+1, len(timeFormats))

	rows := []settingsRow{
		{"Theme", themeDisplay, false, ""},
		{"Default Identity", "", true, s.identityInput.View()},
		{"Poll Interval", "", true, s.intervalInput.View()},
		{"Max History", "", true, s.historyInput.View()},
		{"Time Format", timeFormatDisplay, false, ""},
	}
```

**Step 8: Update help text for time format field**

In `renderHelp()`, add a case for the time format field. It uses left/right cycling like theme, so the same hint applies. Update the condition:

```go
if s.cursor == settingsFieldTheme || s.cursor == settingsFieldTimeFormat {
```

**Step 9: Build**

Run: `make install`
Expected: Clean build.

**Step 10: Commit**

```
feat: add time format toggle to settings view
```

---

### Task 7: Manual Testing and Polish

**Step 1: Test the full flow**

Run the app: `flo` (or `make install && flo`)

Verify:
- Dashboard view shows graph panel at bottom when cursor is on an interface
- Graphs update as you arrow through rows
- In chart uses green, Out chart uses cyan
- Time axis shows labels (default: relative)
- Stats title shows current/peak/avg values
- Pressing Enter still opens the full detail view
- Detail view also shows time axis and stats

**Step 2: Test settings**

- Press `s` to open settings
- Navigate to Time Format row
- Use left/right to cycle: relative → absolute → both
- Press Enter to save
- Verify graphs reflect the new time format

**Step 3: Test edge cases**

- Very small terminal (< 15 body rows): graphs should hide, table stays full
- No dashboard loaded: no graphs panel
- Interface with no history data: "Waiting for data..." message
- Single data point: chart renders without crashing

**Step 4: Fix any polish issues found during testing**

**Step 5: Commit any fixes**

```
fix: polish dashboard graph panel edge cases
```
