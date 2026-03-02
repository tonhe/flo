package components

import (
	"fmt"
	"math"
	"strings"

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
