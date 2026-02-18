package keys

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	Quit      key.Binding
	Dashboard key.Binding
	Identity  key.Binding
	Edit      key.Binding
	Settings  key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Left      key.Binding
	Right     key.Binding
	Tab       key.Binding
}

// DefaultKeyMap provides the default set of key bindings.
var DefaultKeyMap = KeyMap{
	Up:        key.NewBinding(key.WithKeys("up"), key.WithHelp("up", "up")),
	Down:      key.NewBinding(key.WithKeys("down"), key.WithHelp("down", "down")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Dashboard: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dashboards")),
	Identity:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identities")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Settings:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Left:      key.NewBinding(key.WithKeys("left"), key.WithHelp("left", "left")),
	Right:     key.NewBinding(key.WithKeys("right"), key.WithHelp("right", "right")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
}
