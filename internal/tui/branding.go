// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import "github.com/charmbracelet/lipgloss"

// Brand colors. Fixed hex (not AdaptiveColor) because the wordmark is
// brand identity and shouldn't shift between light/dark terminals.
var (
	brandCyan  = lipgloss.Color("#00FFFF")
	brandEye   = lipgloss.Color("#00CED1") // deeper cyan for the inner 'o'
	brandGreen = lipgloss.Color("#A7FF00")
	brandSlate = lipgloss.Color("#6272A4")
)

// headerBrand renders the persistent brand line shown on the left of
// the status header: `go-steer / c[o]go █`. The headline is short
// enough to fit on the same row as the status info on any reasonable
// terminal width, so we don't waste a viewport row on a separate
// banner.
//
// The trailing block is rendered as a `█` glyph in green rather than a
// space with a green background + padding. Background + padding plays
// poorly with some terminals and lipgloss layout (occasional wrapping
// or the whole header collapsing when JoinHorizontal disagrees with
// the outer Padding). The glyph approach is one column wide, has no
// background to mismatch, and renders identically everywhere.
func headerBrand() string {
	bracket := lipgloss.NewStyle().Foreground(brandCyan).Bold(true)
	c := lipgloss.NewStyle().Foreground(brandCyan).Bold(true).Render("c")
	o := lipgloss.NewStyle().Foreground(brandEye).Bold(true).Render("o")
	g := lipgloss.NewStyle().Foreground(brandGreen).Render("go")
	cursor := lipgloss.NewStyle().Foreground(brandGreen).Bold(true).Render("█")
	org := lipgloss.NewStyle().Foreground(brandSlate).Render("go-steer / ")
	return org + c + bracket.Render("[") + o + bracket.Render("]") + g + " " + cursor
}

// emptyStateHint is shown inside the viewport when the chat history is
// empty. The slate italic matches the brand palette without needing a
// full splash banner.
func emptyStateHint() string {
	return lipgloss.NewStyle().Foreground(brandSlate).Italic(true).
		Render("> Type a message and hit Enter. /help for commands.")
}
