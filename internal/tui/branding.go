// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import "github.com/charmbracelet/lipgloss"

// Brand colors. Fixed hex (not AdaptiveColor) because the wordmark is
// brand identity and shouldn't shift between light/dark terminals.
// brandEye (deep cyan for the inner 'o') and brandGreen (highlight
// for "go" / cursor block) were retired with the brand-header
// simplification — see headerBrand for context.
var (
	brandCyan  = lipgloss.Color("#00FFFF")
	brandSlate = lipgloss.Color("#6272A4")
)

// headerBrand renders the persistent brand line shown on the left of
// the status header: `go-steer / c[o]go █`.
//
// Originally this rendered each colored slice as its own lipgloss
// span (slate "go-steer / ", bold-cyan "c", "[", deep-cyan "o",
// bold-cyan "]", green "go", green "█"). That produced ~7 SGR
// open/close pairs per render — fine on most terminals but reported
// to correlate with rendering glitches on the user's VS Code host
// (random capital letters disappearing, traced to malformed CSI
// sequences). Collapsing to two styled spans (slate prefix + bold-
// cyan brand mark) cuts the SGR density without losing the wordmark.
// We trade the green "go" / deep-cyan inner "o" highlights for
// stability; that's a worthwhile swap until the host renderer is
// happier with denser ANSI.
func headerBrand() string {
	prefix := lipgloss.NewStyle().Foreground(brandSlate).Render("go-steer / ")
	mark := lipgloss.NewStyle().Foreground(brandCyan).Bold(true).Render("c[o]go █")
	return prefix + mark
}

// emptyStateHint is shown inside the viewport when the chat history is
// empty. The slate italic matches the brand palette without needing a
// full splash banner.
func emptyStateHint() string {
	return lipgloss.NewStyle().Foreground(brandSlate).Italic(true).
		Render("> Type a message and hit Enter. /help for commands.")
}
