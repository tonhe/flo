package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/internal/engine"
	"github.com/tonhe/flo/internal/identity"
	"github.com/tonhe/flo/tui/keys"
	"github.com/tonhe/flo/tui/styles"
)

// InterfacePickerAction describes the result of an InterfacePickerModel update.
type InterfacePickerAction int

const (
	InterfacePickerNone      InterfacePickerAction = iota
	InterfacePickerSelected  // user confirmed selection
	InterfacePickerCancelled // user dismissed without selecting
)

// InterfaceDiscoverMsg carries the result of an async SNMP discovery.
type InterfaceDiscoverMsg struct {
	Interfaces []engine.DetailedInterface
	Err        error
}

// InterfaceProgressMsg carries a progress update from the discovery goroutine.
type InterfaceProgressMsg struct {
	Status string
}

// ifPickerSpinnerMsg triggers a spinner frame update.
type ifPickerSpinnerMsg struct{}

var ifPickerSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type ifPickerSortMode int

const (
	ifSortByIndex ifPickerSortMode = iota
	ifSortByName
	ifSortByStatus
	ifSortBySpeed
	ifSortByType
	ifSortModeCount // sentinel for cycling
)

func (s ifPickerSortMode) String() string {
	switch s {
	case ifSortByName:
		return "name"
	case ifSortByStatus:
		return "status"
	case ifSortBySpeed:
		return "speed"
	case ifSortByType:
		return "type"
	default:
		return "index"
	}
}

// InterfacePickerModel is a full-screen two-panel interface browser.
type InterfacePickerModel struct {
	theme styles.Theme
	sty   *styles.Styles
	width int
	height int

	// Data
	allInterfaces []engine.DetailedInterface
	filtered      []int          // indices into allInterfaces
	selected      map[int]bool   // ifIndex -> selected
	preSelected   []string       // interface names to pre-select

	// Navigation
	cursor       int
	scrollOffset int

	// Filter state
	filterInput textinput.Model
	filterMode  bool
	showUpOnly  bool
	sortMode    ifPickerSortMode

	// Loading state
	loading       bool
	loadingErr    error
	loadingStatus string // current phase description
	spinnerFrame  int
	progressCh    chan string // receives status updates from discovery goroutine

	// SNMP connection info
	host     string
	port     int
	identity *identity.Identity
}

// NewInterfacePickerModel creates a new picker and returns a command that
// launches async SNMP discovery.
func NewInterfacePickerModel(
	theme styles.Theme,
	host string,
	port int,
	id *identity.Identity,
	preSelected []string,
) (InterfacePickerModel, tea.Cmd) {
	sty := styles.NewStyles(theme)

	fi := textinput.New()
	fi.Placeholder = "search by name, description, IP, neighbor..."
	fi.CharLimit = 64
	fi.Width = 30

	m := InterfacePickerModel{
		theme:         theme,
		sty:           sty,
		selected:      make(map[int]bool),
		preSelected:   preSelected,
		loading:       true,
		loadingStatus: "Connecting...",
		progressCh:    make(chan string, 8),
		host:          host,
		port:          port,
		identity:      id,
		sortMode:      ifSortByIndex,
		showUpOnly:    true,
		filterInput:   fi,
	}

	cmd := tea.Batch(m.discoverCmd(), ifPickerSpinnerCmd())
	return m, cmd
}

func (m InterfacePickerModel) discoverCmd() tea.Cmd {
	host := m.host
	port := m.port
	id := m.identity
	progressCh := m.progressCh
	return func() tea.Msg {
		progress := func(status string) {
			// Non-blocking send so discovery never stalls on a full channel.
			select {
			case progressCh <- status:
			default:
			}
		}
		ifaces, err := engine.DiscoverDetailedInterfaces(host, port, id, progress)
		return InterfaceDiscoverMsg{Interfaces: ifaces, Err: err}
	}
}

func ifPickerSpinnerCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return ifPickerSpinnerMsg{}
	})
}

// SetSize updates the available terminal dimensions.
func (m *InterfacePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.adjustScroll()
}

// SelectedNames returns the names of all selected interfaces, sorted.
func (m InterfacePickerModel) SelectedNames() []string {
	var names []string
	for _, iface := range m.allInterfaces {
		if m.selected[iface.IfIndex] {
			names = append(names, iface.Name)
		}
	}
	sort.Strings(names)
	return names
}

// Update handles messages and returns the updated model, a command, and action.
func (m InterfacePickerModel) Update(msg tea.Msg) (InterfacePickerModel, tea.Cmd, InterfacePickerAction) {
	// Handle async discovery result
	if dmsg, ok := msg.(InterfaceDiscoverMsg); ok {
		m.loading = false
		if dmsg.Err != nil {
			m.loadingErr = dmsg.Err
			return m, nil, InterfacePickerNone
		}
		m.allInterfaces = dmsg.Interfaces
		m.applyPreSelection()
		m.applyFilter()
		m.adjustScroll()
		return m, nil, InterfacePickerNone
	}

	// Handle spinner tick — advance frame and drain progress channel
	if _, ok := msg.(ifPickerSpinnerMsg); ok {
		if m.loading {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(ifPickerSpinnerFrames)
			// Drain any pending progress updates (use latest)
			for {
				select {
				case status := <-m.progressCh:
					m.loadingStatus = status
				default:
					goto drained
				}
			}
		drained:
			return m, ifPickerSpinnerCmd(), InterfacePickerNone
		}
		return m, nil, InterfacePickerNone
	}

	// Handle window resize
	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wmsg.Width
		m.height = wmsg.Height
		m.adjustScroll()
		return m, nil, InterfacePickerNone
	}

	// Loading state: only escape
	if m.loading {
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(kmsg, keys.DefaultKeyMap.Escape) {
				return m, nil, InterfacePickerCancelled
			}
		}
		return m, nil, InterfacePickerNone
	}

	// Error state: escape cancels, r retries
	if m.loadingErr != nil {
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(kmsg, keys.DefaultKeyMap.Escape):
				return m, nil, InterfacePickerCancelled
			case kmsg.String() == "r":
				m.loading = true
				m.loadingErr = nil
				m.loadingStatus = "Connecting..."
				m.progressCh = make(chan string, 8)
				return m, tea.Batch(m.discoverCmd(), ifPickerSpinnerCmd()), InterfacePickerNone
			}
		}
		return m, nil, InterfacePickerNone
	}

	// Filter mode
	if m.filterMode {
		return m.updateFilterMode(msg)
	}

	// Normal navigation
	return m.updateNormal(msg)
}

func (m InterfacePickerModel) updateNormal(msg tea.Msg) (InterfacePickerModel, tea.Cmd, InterfacePickerAction) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil, InterfacePickerNone
	}

	switch {
	case key.Matches(kmsg, keys.DefaultKeyMap.Escape):
		return m, nil, InterfacePickerCancelled

	case key.Matches(kmsg, keys.DefaultKeyMap.Enter):
		return m, nil, InterfacePickerSelected

	case key.Matches(kmsg, keys.DefaultKeyMap.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		m.adjustScroll()

	case key.Matches(kmsg, keys.DefaultKeyMap.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		m.adjustScroll()

	case kmsg.String() == " ":
		if m.cursor < len(m.filtered) {
			idx := m.filtered[m.cursor]
			ifIdx := m.allInterfaces[idx].IfIndex
			if m.selected[ifIdx] {
				delete(m.selected, ifIdx)
			} else {
				m.selected[ifIdx] = true
			}
		}

	case kmsg.String() == "/":
		m.filterMode = true
		m.filterInput.Focus()

	case kmsg.String() == "u":
		m.showUpOnly = !m.showUpOnly
		m.applyFilter()
		m.cursor = 0
		m.scrollOffset = 0

	case kmsg.String() == "s":
		m.sortMode = (m.sortMode + 1) % ifSortModeCount
		m.applyFilter()

	case kmsg.String() == "a":
		for _, idx := range m.filtered {
			m.selected[m.allInterfaces[idx].IfIndex] = true
		}

	case kmsg.String() == "A":
		for k := range m.selected {
			delete(m.selected, k)
		}
	}

	return m, nil, InterfacePickerNone
}

func (m InterfacePickerModel) updateFilterMode(msg tea.Msg) (InterfacePickerModel, tea.Cmd, InterfacePickerAction) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil, InterfacePickerNone
	}

	switch {
	case key.Matches(kmsg, keys.DefaultKeyMap.Escape):
		m.filterMode = false
		m.filterInput.Blur()
		return m, nil, InterfacePickerNone

	case key.Matches(kmsg, keys.DefaultKeyMap.Enter):
		m.filterMode = false
		m.filterInput.Blur()
		return m, nil, InterfacePickerNone

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		// Live filter
		m.applyFilter()
		m.cursor = 0
		m.scrollOffset = 0
		return m, cmd, InterfacePickerNone
	}
}

func (m *InterfacePickerModel) applyPreSelection() {
	nameSet := make(map[string]bool)
	for _, n := range m.preSelected {
		nameSet[n] = true
	}
	for _, iface := range m.allInterfaces {
		if nameSet[iface.Name] || nameSet[iface.Description] {
			m.selected[iface.IfIndex] = true
		}
	}
}

func (m *InterfacePickerModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.filtered = nil

	for i, iface := range m.allInterfaces {
		if m.showUpOnly && iface.Status != "up" {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(iface.Name + " " + iface.Description + " " + iface.Alias)
			for _, ip := range iface.IPAddresses {
				haystack += " " + ip.Address
			}
			for _, n := range iface.Neighbors {
				haystack += " " + strings.ToLower(n.DeviceID) + " " + strings.ToLower(n.RemotePort)
			}
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		m.filtered = append(m.filtered, i)
	}

	sort.SliceStable(m.filtered, func(a, b int) bool {
		ia := m.allInterfaces[m.filtered[a]]
		ib := m.allInterfaces[m.filtered[b]]
		switch m.sortMode {
		case ifSortByName:
			return ia.Name < ib.Name
		case ifSortByStatus:
			return statusRank(ia.Status) < statusRank(ib.Status)
		case ifSortBySpeed:
			return ia.Speed > ib.Speed
		case ifSortByType:
			return ia.IfType < ib.IfType
		default:
			return ia.IfIndex < ib.IfIndex
		}
	})
}

func statusRank(s string) int {
	switch s {
	case "up":
		return 0
	case "down":
		return 1
	default:
		return 2
	}
}

// adjustScroll ensures the cursor is visible within the scroll window.
func (m *InterfacePickerModel) adjustScroll() {
	visibleRows := m.listVisibleRows()
	listLen := len(m.filtered)

	for {
		topReserve := 0
		if m.scrollOffset > 0 {
			topReserve = 1
		}
		bottomReserve := 0
		if m.scrollOffset+visibleRows < listLen {
			bottomReserve = 1
		}
		usable := visibleRows - topReserve - bottomReserve
		if usable < 1 {
			usable = 1
		}
		if m.cursor >= m.scrollOffset+topReserve+usable {
			m.scrollOffset++
		} else {
			break
		}
	}

	for {
		topReserve := 0
		if m.scrollOffset > 0 {
			topReserve = 1
		}
		if m.cursor < m.scrollOffset+topReserve {
			m.scrollOffset--
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
				break
			}
		} else {
			break
		}
	}
}

func (m InterfacePickerModel) listVisibleRows() int {
	// Reserve: title(1) + blank(1) + filter bar(1) + blank(1) + help(1) = 5
	// Plus up-only indicator if active
	rows := m.height - 5
	if m.showUpOnly {
		rows--
	}
	if rows < 3 {
		rows = 3
	}
	return rows
}

// --- View ---

func (m InterfacePickerModel) View() string {
	if m.loading {
		return m.viewLoading()
	}
	if m.loadingErr != nil {
		return m.viewError()
	}

	theme := m.theme
	bg := theme.Base00

	contentHeight := m.height - 5
	if contentHeight < 3 {
		contentHeight = 3
	}

	leftWidth := m.width * 2 / 5
	if leftWidth < 30 {
		leftWidth = 30
	}
	separatorWidth := 3
	rightWidth := m.width - leftWidth - separatorWidth
	if rightWidth < 35 {
		rightWidth = 35
	}

	leftLines := m.renderInterfaceList(leftWidth, contentHeight)
	rightLines := m.renderDetailPanel(rightWidth)

	for len(leftLines) < contentHeight {
		leftLines = append(leftLines, lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", leftWidth)))
	}
	for len(rightLines) < contentHeight {
		rightLines = append(rightLines, lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", rightWidth)))
	}

	sepStyle := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg)
	sep := sepStyle.Render(" \u2502 ")

	var panelLines []string
	for i := 0; i < contentHeight; i++ {
		left := leftLines[i]
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		panelLines = append(panelLines, left+sep+right)
	}

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	selectedCount := 0
	for _, v := range m.selected {
		if v {
			selectedCount++
		}
	}
	titleLine := titleStyle.Render("  Browse Interfaces") +
		countStyle.Render(fmt.Sprintf("  (%d/%d shown, %d selected, sort: %s)",
			len(m.filtered), len(m.allInterfaces), selectedCount, m.sortMode))

	helpLine := m.renderHelp()

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		"",
		strings.Join(panelLines, "\n"),
		"",
		helpLine,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, content,
		lipgloss.WithWhitespaceBackground(bg))
}

func (m InterfacePickerModel) renderInterfaceList(width, totalHeight int) []string {
	theme := m.theme
	bg := theme.Base00
	normalStyle := lipgloss.NewStyle().Foreground(theme.Base05).Background(bg)
	selectedStyle := lipgloss.NewStyle().Foreground(theme.Base06).Background(bg).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	upStyle := lipgloss.NewStyle().Foreground(theme.Base0B).Background(bg)
	downStyle := lipgloss.NewStyle().Foreground(theme.Base08).Background(bg)
	checkStyle := lipgloss.NewStyle().Foreground(theme.Base0B).Background(bg)
	bgStyle := lipgloss.NewStyle().Background(bg)

	var lines []string

	// Filter bar
	if m.filterMode {
		lines = append(lines, bgStyle.Render("  ")+m.filterInput.View())
	} else {
		filterText := m.filterInput.Value()
		if filterText == "" {
			filterText = "/ to search"
		}
		lines = append(lines, bgStyle.Render("  ")+dimStyle.Render(filterText))
	}

	// Up-only indicator
	if m.showUpOnly {
		lines = append(lines, bgStyle.Render("  ")+dimStyle.Render("[up only - press u to show all]"))
	}

	visibleRows := totalHeight - len(lines)
	if visibleRows < 1 {
		visibleRows = 1
	}
	listLen := len(m.filtered)

	showTop := m.scrollOffset > 0
	showBottom := m.scrollOffset+visibleRows < listLen

	if showTop {
		lines = append(lines, dimStyle.Render(ifPickerPadRight("    \u25b2 more", width)))
		visibleRows--
	}
	if showBottom {
		visibleRows--
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	startIdx := m.scrollOffset
	endIdx := startIdx + visibleRows
	if endIdx > listLen {
		endIdx = listLen
	}

	for i := startIdx; i < endIdx; i++ {
		ifaceIdx := m.filtered[i]
		iface := m.allInterfaces[ifaceIdx]
		isCursor := i == m.cursor
		isChecked := m.selected[iface.IfIndex]

		// Checkbox
		check := bgStyle.Render("[ ] ")
		if isChecked {
			check = checkStyle.Render("[x]") + bgStyle.Render(" ")
		}

		// Cursor
		prefix := bgStyle.Render("  ")
		if isCursor {
			prefix = cursorStyle.Render("> ")
		}

		// Status dot
		statusDot := bgStyle.Render("\u2022")
		switch iface.Status {
		case "up":
			statusDot = upStyle.Render("\u2022")
		case "down":
			statusDot = downStyle.Render("\u2022")
		}

		// Name (truncated)
		nameWidth := width - 12
		if nameWidth < 10 {
			nameWidth = 10
		}
		name := iface.Name
		if len(name) > nameWidth {
			name = name[:nameWidth-3] + "..."
		}

		var styledName string
		if isCursor {
			styledName = selectedStyle.Render(ifPickerPadRight(name, nameWidth))
		} else {
			styledName = normalStyle.Render(ifPickerPadRight(name, nameWidth))
		}

		line := prefix + check + statusDot + bgStyle.Render(" ") + styledName
		lines = append(lines, line)
	}

	if showBottom {
		lines = append(lines, dimStyle.Render(ifPickerPadRight("    \u25bc more", width)))
	}

	return lines
}

func (m InterfacePickerModel) renderDetailPanel(width int) []string {
	theme := m.theme
	bg := theme.Base00
	labelStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	valStyle := lipgloss.NewStyle().Foreground(theme.Base06).Background(bg)
	headerStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)
	upStyle := lipgloss.NewStyle().Foreground(theme.Base0B).Background(bg)
	downStyle := lipgloss.NewStyle().Foreground(theme.Base08).Background(bg)
	sectionStyle := lipgloss.NewStyle().Foreground(theme.Base0E).Background(bg).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg)
	sepStyle := lipgloss.NewStyle().Foreground(theme.Base03).Background(bg)

	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return []string{dimStyle.Render("  No interface selected")}
	}

	ifaceIdx := m.filtered[m.cursor]
	iface := m.allInterfaces[ifaceIdx]

	var lines []string

	// Header
	lines = append(lines, headerStyle.Render(" "+iface.Name))
	lines = append(lines, sepStyle.Render(strings.Repeat("\u2500", width)))

	row := func(label, value string) string {
		return " " + labelStyle.Render(ifPickerPadRight(label+":", 16)) + valStyle.Render(value)
	}

	lines = append(lines, row("Index", fmt.Sprintf("%d", iface.IfIndex)))
	if iface.Description != "" {
		lines = append(lines, row("Description", ifPickerTruncate(iface.Description, width-18)))
	}
	if iface.Alias != "" {
		lines = append(lines, row("Alias", ifPickerTruncate(iface.Alias, width-18)))
	}

	// Oper status with color
	operStr := iface.Status
	switch iface.Status {
	case "up":
		operStr = upStyle.Render("up")
	case "down":
		operStr = downStyle.Render("down")
	}
	lines = append(lines, " "+labelStyle.Render(ifPickerPadRight("Oper Status:", 16))+operStr)

	// Admin status with color
	adminStr := iface.AdminStatus
	switch iface.AdminStatus {
	case "up":
		adminStr = upStyle.Render("up")
	case "down":
		adminStr = downStyle.Render("down")
	}
	lines = append(lines, " "+labelStyle.Render(ifPickerPadRight("Admin Status:", 16))+adminStr)

	if iface.IfTypeName != "" {
		lines = append(lines, row("Type", iface.IfTypeName))
	}
	lines = append(lines, row("Speed", ifPickerFormatSpeed(iface.Speed)))
	if iface.MTU > 0 {
		lines = append(lines, row("MTU", fmt.Sprintf("%d", iface.MTU)))
	}
	if iface.MACAddress != "" {
		lines = append(lines, row("MAC Address", iface.MACAddress))
	}

	// IP Addresses
	if len(iface.IPAddresses) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionStyle.Render(" IP Addresses"))
		for _, ip := range iface.IPAddresses {
			display := ip.Address
			if ip.Mask != "" {
				display += " / " + ip.Mask
			}
			lines = append(lines, "   "+valStyle.Render(display))
		}
	}

	// Neighbors
	if len(iface.Neighbors) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionStyle.Render(" Neighbors"))
		for _, n := range iface.Neighbors {
			line := fmt.Sprintf("   [%s] %s", n.Protocol, n.DeviceID)
			if n.RemotePort != "" {
				line += " via " + n.RemotePort
			}
			if n.Platform != "" {
				line += " (" + n.Platform + ")"
			}
			lines = append(lines, valStyle.Render(line))
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render(" No CDP/LLDP neighbors"))
	}

	return lines
}

func (m InterfacePickerModel) renderHelp() string {
	theme := m.theme
	bg := theme.Base00
	helpStyle := lipgloss.NewStyle().Foreground(theme.Base04).Background(bg)
	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Background(bg).Bold(true)

	return helpStyle.Render("  ") +
		keyStyle.Render("\u2191") + helpStyle.Render("/") + keyStyle.Render("\u2193") + helpStyle.Render(" navigate   ") +
		keyStyle.Render("space") + helpStyle.Render(" toggle   ") +
		keyStyle.Render("/") + helpStyle.Render(" filter   ") +
		keyStyle.Render("u") + helpStyle.Render(" up-only   ") +
		keyStyle.Render("s") + helpStyle.Render(" sort   ") +
		keyStyle.Render("a") + helpStyle.Render("/") + keyStyle.Render("A") + helpStyle.Render(" all/none   ") +
		keyStyle.Render("enter") + helpStyle.Render(" confirm   ") +
		keyStyle.Render("esc") + helpStyle.Render(" cancel")
}

func (m InterfacePickerModel) viewLoading() string {
	theme := m.theme
	bg := theme.Base00
	spinnerStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(theme.Base05)
	statusStyle := lipgloss.NewStyle().Foreground(theme.Base0C)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04)

	frame := ifPickerSpinnerFrames[m.spinnerFrame%len(ifPickerSpinnerFrames)]
	spinnerLine := spinnerStyle.Render(frame) + titleStyle.Render(" Discovering interfaces...")
	hostLine := dimStyle.Render(fmt.Sprintf("Host: %s:%d", m.host, m.port))

	statusLine := ""
	if m.loadingStatus != "" {
		statusLine = statusStyle.Render(m.loadingStatus)
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		spinnerLine,
		"",
		hostLine,
		statusLine,
		"",
		dimStyle.Render("[esc] cancel"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceBackground(bg))
}

func (m InterfacePickerModel) viewError() string {
	theme := m.theme
	bg := theme.Base00
	errStyle := lipgloss.NewStyle().Foreground(theme.Base08)
	dimStyle := lipgloss.NewStyle().Foreground(theme.Base04)
	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Bold(true)
	content := lipgloss.JoinVertical(lipgloss.Center,
		errStyle.Render("Discovery failed"),
		"",
		dimStyle.Render(m.loadingErr.Error()),
		"",
		dimStyle.Render(keyStyle.Render("[r]")+" retry   "+keyStyle.Render("[esc]")+" cancel"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content,
		lipgloss.WithWhitespaceBackground(bg))
}

// --- Helpers ---

func ifPickerPadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func ifPickerTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func ifPickerFormatSpeed(mbps uint64) string {
	switch {
	case mbps == 0:
		return "unknown"
	case mbps >= 1000000:
		return fmt.Sprintf("%d Tbps", mbps/1000000)
	case mbps >= 1000:
		return fmt.Sprintf("%d Gbps", mbps/1000)
	default:
		return fmt.Sprintf("%d Mbps", mbps)
	}
}
