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
// Every style explicitly sets a Background so that on Windows Terminal
// (and other terminals where ANSI background inheritance is unreliable)
// the theme background shows through instead of the terminal's default.
func NewStyles(theme Theme) *Styles {
	bg := theme.Base00  // body background
	hbg := theme.Base01 // header/footer bar background

	return &Styles{
		AppContainer: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(bg),

		Header: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(hbg).
			Bold(true).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Background(hbg).
			Bold(true),
		HeaderStatus: lipgloss.NewStyle().
			Foreground(theme.Base0B).
			Background(hbg),

		Footer: lipgloss.NewStyle().
			Foreground(theme.Base04).
			Background(hbg).
			Padding(0, 1),
		FooterKey: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Background(hbg).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Foreground(theme.Base04).
			Background(hbg),

		TableHeader: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Background(bg).
			Bold(true),
		TableRow: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(bg),
		TableRowSel: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base01),
		TableCellDim: lipgloss.NewStyle().
			Foreground(theme.Base03).
			Background(bg),

		StatusUp: lipgloss.NewStyle().
			Foreground(theme.Base0B).
			Background(bg),
		StatusDown: lipgloss.NewStyle().
			Foreground(theme.Base08).
			Background(bg),
		StatusWarn: lipgloss.NewStyle().
			Foreground(theme.Base0A).
			Background(bg),

		UtilLow: lipgloss.NewStyle().
			Foreground(theme.Base0B).
			Background(bg),
		UtilMid: lipgloss.NewStyle().
			Foreground(theme.Base0A).
			Background(bg),
		UtilHigh: lipgloss.NewStyle().
			Foreground(theme.Base08).
			Background(bg),

		SparklineStyle: lipgloss.NewStyle().
			Foreground(theme.Base0C).
			Background(bg),

		GroupHeader: lipgloss.NewStyle().
			Foreground(theme.Base0E).
			Background(bg).
			Bold(true),

		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Base0D).
			BorderBackground(bg).
			Background(bg).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Background(bg).
			Bold(true),

		FormLabel: lipgloss.NewStyle().
			Foreground(theme.Base04).
			Background(bg),
		FormInput: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(bg),
		FormInputActive: lipgloss.NewStyle().
			Foreground(theme.Base06).
			Background(theme.Base02),
		FormCursor: lipgloss.NewStyle().
			Foreground(theme.Base0B).
			Background(bg),

		IdentityName: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Background(bg),
		IdentityVersion: lipgloss.NewStyle().
			Foreground(theme.Base0C).
			Background(bg),
	}
}
