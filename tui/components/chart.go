package components

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// chartBlocks are block characters from empty to full, used for rendering
// the chart area. Index 0 is empty (space), index 8 is full block.
var chartBlocks = []rune{' ', '\u2581', '\u2582', '\u2583', '\u2584', '\u2585', '\u2586', '\u2587', '\u2588'}

// ChartColors holds the color configuration for chart rendering.
type ChartColors struct {
	BarFg   lipgloss.Color // foreground for bar block characters
	LabelFg lipgloss.Color // foreground for Y-axis labels
	TitleFg lipgloss.Color // foreground for the title text
	Bg      lipgloss.Color // background for all chart elements
}

// RenderChart renders an ASCII line chart using block characters.
// data: values to plot (oldest to newest, left to right)
// width: total width in characters (including Y-axis labels)
// height: total height in characters (including title row)
// title: chart title displayed at the top
// colors: themed colors for the chart elements
func RenderChart(data []float64, width, height int, title string, colors ChartColors) string {
	if width < 10 {
		width = 10
	}
	if height < 4 {
		height = 4
	}

	barStyle := lipgloss.NewStyle().Foreground(colors.BarFg).Background(colors.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(colors.LabelFg).Background(colors.Bg)
	titleStyle := lipgloss.NewStyle().Foreground(colors.TitleFg).Background(colors.Bg)
	emptyStyle := lipgloss.NewStyle().Background(colors.Bg)

	// Reserve space: Y-axis label width and title row
	labelWidth := 8 // e.g. "  1.2G "
	chartWidth := width - labelWidth
	if chartWidth < 2 {
		chartWidth = 2
	}
	chartHeight := height - 1 // subtract title row
	if chartHeight < 2 {
		chartHeight = 2
	}

	var lines []string

	// Title row - centered within the full width
	titleLine := titleStyle.Render(centerText(title, width))
	lines = append(lines, titleLine)

	// Handle empty or insufficient data
	if len(data) == 0 {
		for i := 0; i < chartHeight; i++ {
			lines = append(lines, emptyStyle.Render(strings.Repeat(" ", width)))
		}
		return strings.Join(lines, "\n")
	}

	// Trim data to fit chart width
	if len(data) > chartWidth {
		data = data[len(data)-chartWidth:]
	}

	// Build a scale that uses split zones when spikes are extreme
	scale := newChartScale(data, chartHeight)

	// Build the chart grid from top to bottom.
	// Each row represents a range of values. We use block characters to show
	// how much of each cell is "filled" by the data value.
	for row := chartHeight - 1; row >= 0; row-- {
		// Y-axis label: show the value at this row's top boundary
		rowTopVal := scale.valueAtRow(float64(row + 1))
		label := fmt.Sprintf("%7s ", FormatRate(rowTopVal))
		if len(label) > labelWidth {
			label = label[len(label)-labelWidth:]
		}

		var barChars strings.Builder
		var emptyChars strings.Builder

		// Build padding for data that doesn't fill the chart width
		padding := chartWidth - len(data)
		for p := 0; p < padding; p++ {
			emptyChars.WriteRune(' ')
		}

		cellBottom := scale.valueAtRow(float64(row))
		cellTop := rowTopVal
		cellRange := cellTop - cellBottom

		for _, v := range data {
			if cellRange == 0 || v <= cellBottom {
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

	return strings.Join(lines, "\n")
}

// centerText centers s within the given width, padding with spaces.
func centerText(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-len(s)-pad)
}

// ChartOptions holds optional configuration for enhanced chart rendering.
type ChartOptions struct {
	Timestamps []time.Time // timestamps corresponding to data points (for X-axis)
	TimeFormat string      // "relative", "absolute", or "both"
	Label      string      // short label like "In" or "Out" (used in stats title)
}

// FormatTimeLabel formats a timestamp as a time label for the X-axis.
//   - "relative": "now" if <30s ago, else "Xm ago" or "Xh ago" or "Xs ago"
//   - "absolute": ts.Format("15:04")
//   - "both": same as relative, except for the newest point which shows "now HH:MM"
func FormatTimeLabel(ts, now time.Time, format string) string {
	switch format {
	case "absolute":
		return ts.Format("15:04")
	case "both":
		return formatBoth(ts, now)
	default: // "relative"
		return formatRelative(ts, now)
	}
}

// formatRelative returns a relative time label like "now", "30s ago", "5m ago", "1h ago".
func formatRelative(ts, now time.Time) string {
	diff := now.Sub(ts)
	if diff < 10*time.Second {
		return "now"
	}
	if diff < time.Minute {
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(diff.Hours()))
}

// formatBoth returns relative time, but the newest point (< 30s) gets "now HH:MM".
func formatBoth(ts, now time.Time) string {
	diff := now.Sub(ts)
	if diff < 30*time.Second {
		return "now " + ts.Format("15:04")
	}
	return formatRelative(ts, now)
}

// RenderChartWithOptions renders an ASCII chart with optional time axis and stats title.
// Unlike RenderChart, this uses a stats title row and optionally a time axis row at the bottom.
func RenderChartWithOptions(data []float64, width, height int, colors ChartColors, opts ChartOptions) string {
	if width < 10 {
		width = 10
	}
	if height < 4 {
		height = 4
	}

	barStyle := lipgloss.NewStyle().Foreground(colors.BarFg).Background(colors.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(colors.LabelFg).Background(colors.Bg)
	emptyStyle := lipgloss.NewStyle().Background(colors.Bg)

	labelWidth := 8 // e.g. "  1.2G "
	chartWidth := width - labelWidth
	if chartWidth < 2 {
		chartWidth = 2
	}

	// Height budget: title(1) + chart rows + optional time axis(1)
	hasTimeAxis := len(opts.Timestamps) > 0
	chartHeight := height - 1 // subtract title row
	if hasTimeAxis {
		chartHeight-- // subtract time axis row
	}
	if chartHeight < 2 {
		chartHeight = 2
	}

	var lines []string

	// Stats title row
	titleLine := renderStatsTitle(data, width, colors, opts)
	lines = append(lines, titleLine)

	// Handle empty or insufficient data
	if len(data) == 0 {
		for i := 0; i < chartHeight; i++ {
			lines = append(lines, emptyStyle.Render(strings.Repeat(" ", width)))
		}
		if hasTimeAxis {
			lines = append(lines, emptyStyle.Render(strings.Repeat(" ", width)))
		}
		return strings.Join(lines, "\n")
	}

	// Trim data to fit chart width
	trimmedData := data
	if len(trimmedData) > chartWidth {
		trimmedData = trimmedData[len(trimmedData)-chartWidth:]
	}

	// Build a scale that uses split zones when spikes are extreme.
	// Bottom ~80% of rows get full resolution for normal traffic,
	// top ~20% get compressed scale to show spikes without crushing the chart.
	scale := newChartScale(trimmedData, chartHeight)

	// Build the chart grid from top to bottom
	for row := chartHeight - 1; row >= 0; row-- {
		rowTopVal := scale.valueAtRow(float64(row + 1))
		label := fmt.Sprintf("%7s ", FormatRate(rowTopVal))
		if len(label) > labelWidth {
			label = label[len(label)-labelWidth:]
		}

		var barChars strings.Builder
		var emptyChars strings.Builder

		padding := chartWidth - len(trimmedData)
		for p := 0; p < padding; p++ {
			emptyChars.WriteRune(' ')
		}

		cellBottom := scale.valueAtRow(float64(row))
		cellTop := rowTopVal
		cellRange := cellTop - cellBottom

		for _, v := range trimmedData {
			if cellRange == 0 || v <= cellBottom {
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

	// Time axis row (optional)
	if hasTimeAxis {
		// Trim timestamps to match trimmed data
		timestamps := opts.Timestamps
		if len(timestamps) > chartWidth {
			timestamps = timestamps[len(timestamps)-chartWidth:]
		}
		// Ensure timestamps length matches data length
		if len(timestamps) > len(trimmedData) {
			timestamps = timestamps[len(timestamps)-len(trimmedData):]
		}
		axisLine := renderTimeAxis(timestamps, chartWidth, labelWidth, width, opts.TimeFormat, labelStyle)
		lines = append(lines, axisLine)
	}

	return strings.Join(lines, "\n")
}

// renderStatsTitle builds the stats title line: "  In: 1.2G  peak: 3.8G  avg: 1.1G"
func renderStatsTitle(data []float64, width int, colors ChartColors, opts ChartOptions) string {
	barStyle := lipgloss.NewStyle().Foreground(colors.BarFg).Background(colors.Bg)
	labelStyle := lipgloss.NewStyle().Foreground(colors.LabelFg).Background(colors.Bg)
	bgStyle := lipgloss.NewStyle().Background(colors.Bg)

	if len(data) == 0 || opts.Label == "" {
		// No data or no label: render an empty title row
		return bgStyle.Render(strings.Repeat(" ", width))
	}

	// Calculate current (last), peak, avg
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

	// Build the stats title: "  In: 1.2G  peak: 3.8G  avg: 1.1G"
	titleParts := barStyle.Render("  "+opts.Label+": "+FormatRate(current)) +
		labelStyle.Render("  peak: "+FormatRate(peak)) +
		labelStyle.Render("  avg: "+FormatRate(avg))

	// Pad to full width
	titleWidth := lipgloss.Width(titleParts)
	if titleWidth < width {
		titleParts += bgStyle.Render(strings.Repeat(" ", width-titleWidth))
	}

	return titleParts
}

// timeAxisIntervals are the candidate intervals for time axis labels,
// ordered from smallest to largest. The algorithm picks the smallest
// interval that avoids label overlap.
var timeAxisIntervals = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	15 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	1 * time.Hour,
}

// renderTimeAxis builds the time axis row with time-anchored labels.
// "now" is always pinned to the right edge. Round-time markers (e.g. 30s,
// 1m, 2m) appear at their natural positions and scroll left as data grows.
func renderTimeAxis(timestamps []time.Time, chartWidth, labelWidth, totalWidth int, format string, labelStyle lipgloss.Style) string {
	if len(timestamps) == 0 {
		return labelStyle.Render(strings.Repeat(" ", totalWidth))
	}

	now := time.Now()

	// Build the axis line as a character buffer
	axis := make([]byte, chartWidth)
	for i := range axis {
		axis[i] = ' '
	}

	// Data offset: empty columns before data starts
	dataOffset := chartWidth - len(timestamps)
	if dataOffset < 0 {
		dataOffset = 0
	}

	// Always place "now" at the rightmost data position
	nowLabel := FormatTimeLabel(timestamps[len(timestamps)-1], now, format)
	nowPos := chartWidth - len(nowLabel)
	if nowPos < 0 {
		nowPos = 0
	}
	for j := 0; j < len(nowLabel) && nowPos+j < chartWidth; j++ {
		axis[nowPos+j] = nowLabel[j]
	}
	// Track occupied regions to prevent overlap (need 2-char gap between labels)
	type region struct{ start, end int }
	occupied := []region{{nowPos, chartWidth - 1}}

	// Determine the time span of the data
	oldest := timestamps[0]
	span := now.Sub(oldest)
	if span < time.Second {
		prefix := strings.Repeat(" ", labelWidth)
		return labelStyle.Render(prefix + string(axis))
	}

	// Pick the smallest interval that yields a reasonable label count.
	// We want labels spaced at least ~10 chars apart.
	interval := timeAxisIntervals[len(timeAxisIntervals)-1]
	for _, candidate := range timeAxisIntervals {
		if candidate > span {
			continue
		}
		// Estimate how many labels this interval would produce
		count := int(span / candidate)
		if count <= 0 {
			continue
		}
		// Estimate average spacing in chars
		spacing := chartWidth / count
		if spacing >= 10 {
			interval = candidate
			break
		}
	}

	// Place labels at each interval mark, working from right (newest) to left.
	// Start from the first full interval before "now".
	for elapsed := interval; elapsed <= span; elapsed += interval {
		label := formatElapsed(elapsed)

		// Find the data index closest to this elapsed time
		targetTime := now.Add(-elapsed)
		dataIdx := findClosestTimestamp(timestamps, targetTime)
		pos := dataOffset + dataIdx

		// Center the label on this position
		startPos := pos - len(label)/2
		if startPos < 0 {
			startPos = 0
		}
		if startPos+len(label) > chartWidth {
			startPos = chartWidth - len(label)
		}
		if startPos < 0 {
			continue
		}
		endPos := startPos + len(label) - 1

		// Check overlap with all occupied regions (need 2-char gap)
		overlaps := false
		for _, r := range occupied {
			if startPos <= r.end+2 && endPos >= r.start-2 {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		// Write label into axis buffer
		for j := 0; j < len(label) && startPos+j < chartWidth; j++ {
			axis[startPos+j] = label[j]
		}
		occupied = append(occupied, region{startPos, endPos})
	}

	prefix := strings.Repeat(" ", labelWidth)
	return labelStyle.Render(prefix + string(axis))
}

// formatElapsed formats a duration as a compact label: "10s", "1.5m", "5m", "1h".
// Fractional minutes are shown with up to one decimal place (e.g. 90s → "1.5m"),
// but trailing ".0" is omitted for whole minutes (e.g. 60s → "1m").
func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		totalSecs := int(d.Seconds())
		if totalSecs%60 == 0 {
			return fmt.Sprintf("%dm", totalSecs/60)
		}
		// Fractional minutes: trim trailing zeros after decimal
		s := fmt.Sprintf("%.1f", float64(totalSecs)/60.0)
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		return s + "m"
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// findClosestTimestamp returns the index of the timestamp closest to target.
func findClosestTimestamp(timestamps []time.Time, target time.Time) int {
	best := 0
	bestDiff := absDuration(timestamps[0].Sub(target))
	for i := 1; i < len(timestamps); i++ {
		diff := absDuration(timestamps[i].Sub(target))
		if diff < bestDiff {
			bestDiff = diff
			best = i
		}
	}
	return best
}

// absDuration returns the absolute value of a duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// chartScale handles Y-axis scaling with an optional compressed spike zone.
// When data has extreme spikes (actualMax > 3x p95), the chart splits into:
//   - Normal zone (bottom ~80% of rows): linear scale from 0 to breakpoint
//   - Spike zone (top ~20% of rows): compressed linear scale from breakpoint to max
//
// This keeps normal traffic readable while still showing spikes.
type chartScale struct {
	max        float64 // actual data maximum
	breakpoint float64 // split point between zones (0 = no split, pure linear)
	normalRows int     // rows allocated to the normal zone
	totalRows  int     // total chart rows
}

// newChartScale analyzes data and returns a scale, splitting into two zones
// when extreme spikes are detected.
func newChartScale(data []float64, totalRows int) chartScale {
	s := chartScale{totalRows: totalRows, normalRows: totalRows}

	if len(data) == 0 {
		s.max = 1
		return s
	}

	// Find actual max
	actualMax := data[0]
	for _, v := range data[1:] {
		if v > actualMax {
			actualMax = v
		}
	}
	if actualMax == 0 {
		actualMax = 1
	}
	s.max = actualMax

	// Need enough data points for meaningful percentile
	if len(data) <= 3 {
		return s
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	median := sorted[len(sorted)/2]

	// Split when the max is dramatically higher than typical traffic.
	if median == 0 || actualMax <= median*5 {
		return s
	}

	// Find the breakpoint by locating the largest RELATIVE gap in the sorted
	// data. Using ratio (sorted[i]/sorted[i-1]) instead of absolute difference
	// ensures we find the transition from normal traffic to spikes, not gaps
	// between spike sub-clusters. E.g., 3M→94M (31x) beats 94M→237M (2.5x).
	maxRelGap := 0.0
	gapIdx := 0
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] <= 0 {
			continue
		}
		relGap := sorted[i] / sorted[i-1]
		if relGap > maxRelGap {
			maxRelGap = relGap
			gapIdx = i
		}
	}

	// Place breakpoint just above the normal cluster, with headroom
	if gapIdx > 0 {
		s.breakpoint = sorted[gapIdx-1] * 1.3
	}
	if s.breakpoint < 1 {
		s.breakpoint = sorted[gapIdx] * 0.1
	}
	s.normalRows = int(float64(totalRows) * 0.8)
	if s.normalRows < 2 {
		s.normalRows = 2
	}
	if s.normalRows >= totalRows {
		s.normalRows = totalRows - 1
	}

	return s
}

// valueAtRow returns the data value at a given row boundary.
// Row 0 bottom = 0, row totalRows top = max.
// Uses piecewise linear mapping when split-scale is active.
func (s chartScale) valueAtRow(row float64) float64 {
	if s.breakpoint == 0 {
		// Pure linear
		return s.max * row / float64(s.totalRows)
	}

	nr := float64(s.normalRows)
	if row <= nr {
		// Normal zone: 0 to breakpoint
		return s.breakpoint * row / nr
	}

	// Spike zone: breakpoint to max
	spikeRows := float64(s.totalRows - s.normalRows)
	return s.breakpoint + (s.max-s.breakpoint)*(row-nr)/spikeRows
}
