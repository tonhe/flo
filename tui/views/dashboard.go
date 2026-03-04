package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/tui/components"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// Column width constants (minimum widths).
const (
	colDevice    = 16
	colInterface = 18
	colStatus    = 8
	colIn        = 10
	colOut       = 10
	colUtil      = 8
	colSparkMin  = 12
)

// Layout constants for split view (table + graph panel).
const (
	minTableRows = 6 // 1 header + 5 data rows
	minGraphRows = 8 // 1 title + 6 chart + 1 time axis
)

// DashboardView is the main monitoring table view showing all interface
// metrics grouped by target group.
type DashboardView struct {
	theme      styles.Theme
	sty        *styles.Styles
	snapshot   *engine.DashboardSnapshot
	cursor     int
	width      int
	height     int
	totalRows  int
	offset     int // scroll offset for vertical scrolling
	timeFormat string
}

// NewDashboardView creates a new DashboardView with the given theme.
func NewDashboardView(theme styles.Theme) DashboardView {
	return DashboardView{
		theme:      theme,
		sty:        styles.NewStyles(theme),
		timeFormat: "relative",
	}
}

// SetTimeFormat updates the time format used for chart time axis labels.
func (v *DashboardView) SetTimeFormat(format string) {
	v.timeFormat = format
}

// Update handles key messages for cursor navigation within the dashboard.
func (v DashboardView) Update(msg tea.Msg) (DashboardView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.DefaultKeyMap.Up):
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
		case key.Matches(msg, keys.DefaultKeyMap.Down):
			if v.cursor < v.totalRows-1 {
				v.cursor++
				v.ensureVisible()
			}
		}
	}
	return v, nil
}

// SetSnapshot updates the dashboard data. It recalculates the total row count
// and clamps the cursor if needed.
func (v *DashboardView) SetSnapshot(snap *engine.DashboardSnapshot) {
	v.snapshot = snap
	total := 0
	if snap != nil {
		for _, g := range snap.Groups {
			for _, t := range g.Targets {
				total += len(t.Interfaces)
			}
		}
	}
	v.totalRows = total
	if v.cursor >= v.totalRows && v.totalRows > 0 {
		v.cursor = v.totalRows - 1
	}
}

// SetSize updates the available dimensions for the view.
func (v *DashboardView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// Cursor returns the current cursor position (flat interface index).
func (v DashboardView) Cursor() int {
	return v.cursor
}

// SelectedInterface returns the target label and InterfaceStats at the current
// cursor position, or empty values if nothing is selected.
func (v DashboardView) SelectedInterface() (label string, iface *engine.InterfaceStats) {
	if v.snapshot == nil {
		return "", nil
	}
	idx := 0
	for _, g := range v.snapshot.Groups {
		for _, t := range g.Targets {
			for i := range t.Interfaces {
				if idx == v.cursor {
					return t.Label, &t.Interfaces[i]
				}
				idx++
			}
		}
	}
	return "", nil
}

// View renders the dashboard view with an optional graph panel below the table.
func (v DashboardView) View() string {
	if v.snapshot == nil || len(v.snapshot.Groups) == 0 {
		return v.renderEmpty()
	}

	// Calculate layout split
	graphHeight := 0
	separatorHeight := 0
	tableHeight := v.height

	// Only show graphs if we have enough room for both
	if v.height >= minTableRows+minGraphRows+1 {
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

	tableContent := v.renderTableWithHeight(tableHeight)

	if graphHeight == 0 {
		return tableContent
	}

	sepStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base03).
		Background(v.theme.Base00)
	separator := sepStyle.Render(strings.Repeat("─", v.width))

	graphPanel := v.renderGraphPanel(graphHeight)

	return lipgloss.JoinVertical(lipgloss.Left, tableContent, separator, graphPanel)
}

// ensureVisible adjusts the scroll offset so the cursor row is visible.
func (v *DashboardView) ensureVisible() {
	// Calculate effective table height, accounting for graph panel.
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

	visible := tableHeight - 1
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

// columnWidths calculates responsive column widths based on terminal width.
// The sparkline column gets all remaining space.
func (v DashboardView) columnWidths() (device, iface, status, inCol, outCol, util, spark int) {
	device = colDevice
	iface = colInterface
	status = colStatus
	inCol = colIn
	outCol = colOut
	util = colUtil

	fixed := device + iface + status + inCol + outCol + util
	spark = v.width - fixed
	if spark < colSparkMin {
		spark = colSparkMin
	}
	return
}

// renderTableWithHeight renders the full dashboard table with group headers and
// interface rows, constrained to the given height.
func (v DashboardView) renderTableWithHeight(tableHeight int) string {
	wDevice, wIface, wStatus, wIn, wOut, wUtil, wSpark := v.columnWidths()

	var lines []string

	// Table header row
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

	// Build all content rows (group headers + interface rows).
	// We track a flat row index for cursor matching.
	type row struct {
		isGroup bool
		text    string
	}
	var rows []row

	rowIdx := 0
	for _, g := range v.snapshot.Groups {
		// Group header
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

	// Apply vertical scrolling. The offset is relative to the flat list of
	// all rows (group headers + interface rows combined).
	// We need to map the interface-only cursor offset to the combined list.
	// Recalculate offset in terms of the combined rows list.
	visibleHeight := tableHeight - 1 // subtract header
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Find the combined-list index of the cursor row
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

	// Calculate scroll window in combined-row space
	startIdx := 0
	if cursorCombinedIdx >= visibleHeight {
		startIdx = cursorCombinedIdx - visibleHeight + 1
	}
	// Also respect the stored offset: if the cursor is near the top, don't
	// scroll past visible rows.
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

	// Pad to fill allocated table height so the graph panel stays pinned
	// to the bottom of the screen.
	emptyRow := lipgloss.NewStyle().Background(v.theme.Base00).Render(strings.Repeat(" ", v.width))
	for len(lines) < tableHeight {
		lines = append(lines, emptyRow)
	}

	return strings.Join(lines, "\n")
}

// renderInterfaceRow renders a single interface metrics row.
func (v DashboardView) renderInterfaceRow(
	deviceLabel string,
	iface engine.InterfaceStats,
	wDevice, wIface, wStatus, wIn, wOut, wUtil, wSpark int,
	selected bool,
) string {
	// Base row style (normal or selected)
	rowStyle := v.sty.TableRow
	if selected {
		rowStyle = v.sty.TableRowSel
	}

	// Device label
	device := rowStyle.Render(padRight(truncate(deviceLabel, wDevice-1), wDevice))

	// Interface name
	ifName := rowStyle.Render(padRight(truncate(iface.Name, wIface-1), wIface))

	// Status with color
	notPolled := iface.Status == ""
	var statusStr string
	switch iface.Status {
	case "":
		st := lipgloss.NewStyle().Foreground(v.theme.Base04)
		if selected {
			st = st.Background(v.theme.Base02)
		}
		statusStr = st.Render(padRight("...", wStatus))
	case "up":
		st := v.sty.StatusUp
		if selected {
			st = st.Background(v.theme.Base02)
		}
		statusStr = st.Render(padRight("up", wStatus))
	case "down":
		st := v.sty.StatusDown
		if selected {
			st = st.Background(v.theme.Base02)
		}
		statusStr = st.Render(padRight("down", wStatus))
	default:
		st := v.sty.StatusWarn
		if selected {
			st = st.Background(v.theme.Base02)
		}
		statusStr = st.Render(padRight(iface.Status, wStatus))
	}

	// In/Out rates
	var inStr, outStr string
	if notPolled {
		inStr = rowStyle.Render(padLeft("---", wIn))
		outStr = rowStyle.Render(padLeft("---", wOut))
	} else {
		inStr = rowStyle.Render(padLeft(components.FormatRate(iface.InRate), wIn))
		outStr = rowStyle.Render(padLeft(components.FormatRate(iface.OutRate), wOut))
	}

	// Utilization with threshold coloring
	var utilStr string
	if notPolled {
		utilStr = rowStyle.Render(padLeft("---", wUtil))
	} else {
		utilText := fmt.Sprintf("%.1f%%", iface.Utilization)
		switch {
		case iface.Utilization >= 80:
			st := v.sty.UtilHigh
			if selected {
				st = st.Background(v.theme.Base02)
			}
			utilStr = st.Render(padLeft(utilText, wUtil))
		case iface.Utilization >= 50:
			st := v.sty.UtilMid
			if selected {
				st = st.Background(v.theme.Base02)
			}
			utilStr = st.Render(padLeft(utilText, wUtil))
		default:
			st := v.sty.UtilLow
			if selected {
				st = st.Background(v.theme.Base02)
			}
			utilStr = st.Render(padLeft(utilText, wUtil))
		}
	}

	// Sparkline from history
	sparkData := extractSparkData(iface.History, wSpark)
	sparkStr := components.Sparkline(sparkData, wSpark)
	sparkStyle := v.sty.SparklineStyle
	if selected {
		sparkStyle = sparkStyle.Background(v.theme.Base02)
	}
	sparkRendered := sparkStyle.Render(sparkStr)

	return fmt.Sprintf("%s%s%s%s%s%s%s",
		device, ifName, statusStr, inStr, outStr, utilStr, sparkRendered,
	)
}

// renderGraphPanel renders the In/Out traffic charts for the selected interface.
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

	sepStyle := lipgloss.NewStyle().Foreground(v.theme.Base03).Background(v.theme.Base00)
	sepLines := make([]string, chartHeight)
	for i := range sepLines {
		sepLines[i] = sepStyle.Render(" | ")
	}
	sep := strings.Join(sepLines, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, inChart, sep, outChart)
}

// renderEmpty renders a centered message when no dashboard is loaded.
func (v DashboardView) renderEmpty() string {
	msgStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base04).
		Align(lipgloss.Center)

	keyStyle := lipgloss.NewStyle().
		Foreground(v.theme.Base0D).
		Bold(true)

	msg := lipgloss.JoinVertical(lipgloss.Center,
		"",
		msgStyle.Render("No dashboard loaded"),
		"",
		msgStyle.Render(fmt.Sprintf(
			"Press %s to open or create a dashboard",
			keyStyle.Render("[d]"),
		)),
		"",
	)

	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, msg,
		lipgloss.WithWhitespaceBackground(v.theme.Base00))
}

// extractSparkData pulls utilization values from the history ring buffer
// for sparkline rendering.
func extractSparkData(history *engine.RingBuffer[engine.RateSample], maxWidth int) []float64 {
	if history == nil {
		return nil
	}
	samples := history.All()
	if len(samples) == 0 {
		return nil
	}

	data := make([]float64, len(samples))
	for i, s := range samples {
		// Use max of in/out for sparkline
		rate := s.InRate
		if s.OutRate > rate {
			rate = s.OutRate
		}
		data[i] = rate
	}

	if len(data) > maxWidth {
		data = data[len(data)-maxWidth:]
	}
	return data
}

// padRight pads s with spaces on the right to the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads s with spaces on the left to the given width.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// truncate shortens s to maxLen characters, adding an ellipsis if needed.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
