package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the application
type KeyMap struct {
	Up             key.Binding
	Down           key.Binding
	Left           key.Binding
	Right          key.Binding
	Enter          key.Binding
	Back           key.Binding
	Tab            key.Binding
	Logs           key.Binding
	Shell          key.Binding
	Describe       key.Binding
	Delete         key.Binding
	AllNS          key.Binding
	Search         key.Binding
	CommandPalette key.Binding
	ContextSwitch  key.Binding
	BackendSwitch  key.Binding
	Start          key.Binding
	Stop           key.Binding
	Restart        key.Binding
	Quit           key.Binding
	Help           key.Binding
}

// DefaultKeyMap returns the default key bindings
var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Logs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	Shell: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "shell"),
	),
	Describe: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "describe"),
	),
	Delete: key.NewBinding(
		key.WithKeys("delete", "backspace"),
		key.WithHelp("del", "delete"),
	),
	AllNS: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "all namespaces"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	CommandPalette: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "command palette"),
	),
	ContextSwitch: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "switch context"),
	),
	BackendSwitch: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "toggle docker/k8s"),
	),
	Start: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "start container"),
	),
	Stop: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "stop container"),
	),
	Restart: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "restart container"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// ShortHelp returns keybindings for the short help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Quit}
}

// FullHelp returns keybindings for the full help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Tab},
		{k.Logs, k.Shell, k.Describe, k.Delete},
		{k.Start, k.Stop, k.Restart},
		{k.AllNS, k.Search, k.CommandPalette},
		{k.ContextSwitch, k.BackendSwitch, k.Quit, k.Help},
	}
}
