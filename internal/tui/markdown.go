// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import "github.com/charmbracelet/glamour"

// MarkdownRenderer wraps a Glamour TermRenderer to render assistant
// messages on TurnComplete. Streaming partial chunks bypass this
// renderer and display as plain text — Glamour can't gracefully render
// half-formed code fences or tables.
type MarkdownRenderer struct {
	r *glamour.TermRenderer
}

// NewMarkdownRenderer constructs a renderer with a fixed style name and
// word wrap at width characters. Width <= 0 disables wrap.
//
// styleName must be a recognized Glamour style ("dark", "light",
// "notty", etc.). We deliberately avoid glamour.WithAutoStyle() here:
// it issues an OSC-11 background-color query to the terminal every
// call, and once Bubble Tea is reading stdin, the terminal's response
// races into the textarea as input. The TUI detects light vs dark
// once at startup (before tea.NewProgram) and threads the result
// through this constructor.
//
// Returns a usable (no-op) renderer plus a non-nil error if Glamour
// initialization fails so the TUI can keep running with raw markdown
// rather than crashing.
func NewMarkdownRenderer(width int, styleName string) (*MarkdownRenderer, error) {
	if styleName == "" {
		styleName = "dark"
	}
	opts := []glamour.TermRendererOption{
		glamour.WithStandardStyle(styleName),
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
