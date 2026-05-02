package tui

import "github.com/charmbracelet/glamour"

// MarkdownRenderer wraps a Glamour TermRenderer to render assistant
// messages on TurnComplete. Streaming partial chunks bypass this
// renderer and display as plain text — Glamour can't gracefully render
// half-formed code fences or tables.
type MarkdownRenderer struct {
	r *glamour.TermRenderer
}

// NewMarkdownRenderer constructs a renderer with auto-detected terminal
// styling and word wrap at width characters. Width <= 0 disables wrap.
//
// Returns a usable (no-op) renderer plus a non-nil error if Glamour
// initialization fails so the TUI can keep running with raw markdown
// rather than crashing.
func NewMarkdownRenderer(width int) (*MarkdownRenderer, error) {
	opts := []glamour.TermRendererOption{
		glamour.WithAutoStyle(),
	}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return &MarkdownRenderer{}, err
	}
	return &MarkdownRenderer{r: r}, nil
}

// Render applies Glamour to markdown. If the renderer failed to
// initialize, it returns markdown unchanged so the user still sees
// something usable.
func (m *MarkdownRenderer) Render(markdown string) string {
	if m == nil || m.r == nil {
		return markdown
	}
	out, err := m.r.Render(markdown)
	if err != nil {
		return markdown
	}
	return out
}
