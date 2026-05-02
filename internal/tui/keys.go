package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap centralizes the keybindings the TUI handles directly. Bubble
// Tea's textarea consumes most other keys (typing, arrows, etc.).
type KeyMap struct {
	Submit     key.Binding
	Newline    key.Binding
	Cancel     key.Binding // Ctrl+C — interrupts current turn or exits
	ClearView  key.Binding // Ctrl+L — clears viewport (history preserved)
	ScrollUp   key.Binding
	ScrollDown key.Binding
	LineUp     key.Binding // Up arrow — scrolls viewport when input empty
	LineDown   key.Binding // Down arrow — scrolls viewport when input empty

	// Permission modal: y allow once, n deny, s allow for the session,
	// a allow always (persisted).
	ConfirmAllowOnce    key.Binding
	ConfirmDeny         key.Binding
	ConfirmAllowSession key.Binding
	ConfirmAllowAlways  key.Binding
}

// DefaultKeyMap returns Cogo's V1 bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		Newline: key.NewBinding(
			key.WithKeys("shift+enter", "ctrl+j"),
			key.WithHelp("shift+enter", "newline"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "cancel/exit"),
		),
		ClearView: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear viewport"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "scroll down"),
		),
		LineUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "scroll up (when input empty)"),
		),
		LineDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "scroll down (when input empty)"),
		),
		ConfirmAllowOnce: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "allow once"),
		),
		ConfirmDeny: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "deny"),
		),
		ConfirmAllowSession: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "allow for this session"),
		),
		ConfirmAllowAlways: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "always allow (persist)"),
		),
	}
}
