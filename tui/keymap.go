package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all the keybindings for the TUI. Bubble Tea's key.Binding
// type lets us define multiple keys for the same action (e.g., "j" and "↓"
// both move down) and attach help text that we display in the help bar.
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Tab    key.Binding
	Space  key.Binding
	All    key.Binding
	Search key.Binding
	Export key.Binding
	Enter  key.Binding
	Escape key.Binding
	Quit   key.Binding
}

// keys is the default keybinding configuration.
// key.WithKeys defines which key(s) trigger the binding.
// key.WithHelp defines the short text shown in the help bar.
var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch pane"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "select"),
	),
	All: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "select all"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Export: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "export"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// helpText returns a formatted string of all keybindings for the help bar.
func helpText() string {
	return "  ↑↓/jk navigate  tab switch pane  space select  a all  / search  e export  q quit"
}
