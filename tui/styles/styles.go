package styles

import "github.com/charmbracelet/lipgloss"

// Styles holds all themed lipgloss styles for the application.
type Styles struct {
	// Layout
	AppContainer lipgloss.Style

	// Header / Footer
	Header       lipgloss.Style
	HeaderTitle  lipgloss.Style
	HeaderStatus lipgloss.Style
	Footer       lipgloss.Style
	FooterKey    lipgloss.Style
	FooterDesc   lipgloss.Style

	// Table
	TableHeader  lipgloss.Style
	TableRow     lipgloss.Style
	TableRowSel  lipgloss.Style
	TableCellDim lipgloss.Style

	// Status colors
	StatusUp   lipgloss.Style
	StatusDown lipgloss.Style
	StatusWarn lipgloss.Style

	// Utilization thresholds
	UtilLow  lipgloss.Style // < 50%
	UtilMid  lipgloss.Style // 50-80%
	UtilHigh lipgloss.Style // > 80%

	// Sparkline
	SparklineStyle lipgloss.Style

	// Groups
	GroupHeader lipgloss.Style

	// Modal / overlay
	ModalBorder lipgloss.Style
	ModalTitle  lipgloss.Style

	// Form
	FormLabel       lipgloss.Style
	FormInput       lipgloss.Style
	FormInputActive lipgloss.Style
	FormCursor      lipgloss.Style

	// Identity table
	IdentityName    lipgloss.Style
	IdentityVersion lipgloss.Style
}

// NewStyles creates a new Styles instance from a theme.
func NewStyles(theme Theme) *Styles {
	return &Styles{
		AppContainer: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base00),

		Header: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base01).
			Bold(true).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		HeaderStatus: lipgloss.NewStyle().
			Foreground(theme.Base0B),

		Footer: lipgloss.NewStyle().
			Foreground(theme.Base04).
			Background(theme.Base01).
			Padding(0, 1),
		FooterKey: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Foreground(theme.Base04),

		TableHeader: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		TableRow: lipgloss.NewStyle().
			Foreground(theme.Base05),
		TableRowSel: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base02),
		TableCellDim: lipgloss.NewStyle().
			Foreground(theme.Base03),

		StatusUp: lipgloss.NewStyle().
			Foreground(theme.Base0B),
		StatusDown: lipgloss.NewStyle().
			Foreground(theme.Base08),
		StatusWarn: lipgloss.NewStyle().
			Foreground(theme.Base0A),

		UtilLow: lipgloss.NewStyle().
			Foreground(theme.Base0B),
		UtilMid: lipgloss.NewStyle().
			Foreground(theme.Base0A),
		UtilHigh: lipgloss.NewStyle().
			Foreground(theme.Base08),

		SparklineStyle: lipgloss.NewStyle().
			Foreground(theme.Base0C),

		GroupHeader: lipgloss.NewStyle().
			Foreground(theme.Base0E).
			Bold(true),

		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Base0D).
			BorderBackground(theme.Base00).
			Background(theme.Base00).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),

		FormLabel: lipgloss.NewStyle().
			Foreground(theme.Base04),
		FormInput: lipgloss.NewStyle().
			Foreground(theme.Base05),
		FormInputActive: lipgloss.NewStyle().
			Foreground(theme.Base06).
			Background(theme.Base02),
		FormCursor: lipgloss.NewStyle().
			Foreground(theme.Base0B),

		IdentityName: lipgloss.NewStyle().
			Foreground(theme.Base0D),
		IdentityVersion: lipgloss.NewStyle().
			Foreground(theme.Base0C),
	}
}
