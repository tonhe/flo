package components

import (
	"fmt"
	"math"
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

	// Find min and max for scaling
	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Ensure we have some range to work with
	if maxVal == minVal {
		maxVal = minVal + 1
	}
	// Always start Y-axis at 0 if all values are positive
	if minVal > 0 {
		minVal = 0
	}

	spread := maxVal - minVal

	// Build the chart grid from top to bottom.
	// Each row represents a range of values. We use block characters to show
	// how much of each cell is "filled" by the data value.
	for row := chartHeight - 1; row >= 0; row-- {
		// Y-axis label: show the value at this row's midpoint
		rowTopVal := minVal + spread*float64(row+1)/float64(chartHeight)
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

		for _, v := range data {
			// Calculate how much of this cell the value fills.
			cellBottom := minVal + spread*float64(row)/float64(chartHeight)
			cellTop := minVal + spread*float64(row+1)/float64(chartHeight)
			cellRange := cellTop - cellBottom

			if v <= cellBottom {
				// Value is below this cell
				barChars.WriteRune(' ')
			} else if v >= cellTop {
				// Value fills this entire cell
				barChars.WriteRune(chartBlocks[8])
			} else {
				// Value partially fills this cell
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

// formatRelative returns a relative time label like "now", "5m ago", "1h ago".
func formatRelative(ts, now time.Time) string {
	diff := now.Sub(ts)
	if diff < 30*time.Second {
		return "now"
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

	// Find min and max for scaling
	minVal, maxVal := trimmedData[0], trimmedData[0]
	for _, v := range trimmedData {
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

	// Build the chart grid from top to bottom
	for row := chartHeight - 1; row >= 0; row-- {
		rowTopVal := minVal + spread*float64(row+1)/float64(chartHeight)
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

		for _, v := range trimmedData {
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

// renderTimeAxis builds the time axis row with evenly-spaced time labels.
func renderTimeAxis(timestamps []time.Time, chartWidth, labelWidth, totalWidth int, format string, labelStyle lipgloss.Style) string {
	now := time.Now()

	// Determine number of labels based on chart width
	numLabels := 4
	if chartWidth < 30 {
		numLabels = 2
	} else if chartWidth < 50 {
		numLabels = 3
	}

	if len(timestamps) < numLabels {
		numLabels = len(timestamps)
	}
	if numLabels == 0 {
		return labelStyle.Render(strings.Repeat(" ", totalWidth))
	}

	// Build the axis line as a character buffer (for the chart area)
	axis := make([]byte, chartWidth)
	for i := range axis {
		axis[i] = ' '
	}

	// Calculate padding offset (data may not fill full chart width)
	dataOffset := chartWidth - len(timestamps)
	if dataOffset < 0 {
		dataOffset = 0
	}

	// Place labels at evenly-spaced positions within the data range
	for i := 0; i < numLabels; i++ {
		var dataIdx int
		if numLabels == 1 {
			dataIdx = len(timestamps) - 1
		} else {
			dataIdx = i * (len(timestamps) - 1) / (numLabels - 1)
		}

		label := FormatTimeLabel(timestamps[dataIdx], now, format)

		// Position in the axis buffer
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
			continue // label doesn't fit
		}

		// Write label into axis buffer
		for j := 0; j < len(label) && startPos+j < chartWidth; j++ {
			axis[startPos+j] = label[j]
		}
	}

	// Build the full line: label-width padding + axis
	prefix := strings.Repeat(" ", labelWidth)
	return labelStyle.Render(prefix + string(axis))
}
