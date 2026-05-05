// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import "github.com/charmbracelet/lipgloss"

// thinkingTickInterval is how long each cheeky thinking phrase stays
// on screen before the next one rotates in. Three seconds is long
// enough to read without effort and short enough that the indicator
// still feels alive.
const thinkingTickInterval = 3 * 1000 // milliseconds — see update.go for the tea.Tick wiring

// thinkingPhrases is a curated set of "Thinking…" alternatives shown
// in the chat window while the model composes a response. The first
// entry — plain "Thinking…" — anchors the indicator: a fresh turn
// always starts there so the affordance is unambiguous before the
// rotator wanders into the AI / sci-fi / CS jokes.
var thinkingPhrases = []string{
	"Thinking…",
	"Consulting the latent space…",
	"Sampling from the distribution…",
	"Reticulating splines…",
	"Computing the answer to the ultimate question…",
	"Spinning up the attention heads…",
	"Asking Stack Overflow nicely…",
	"Untangling pointer chains…",
	"Bargaining with the loss function…",
	"Compiling a thoughtful response…",
	"Defragmenting cache lines…",
	"Negotiating with the Vogons…",
	"Brewing a fresh stack frame…",
	"Plotting a hyperspace course…",
	"Resolving promises…",
	"Eval'ing your prompt…",
}

// thinkingPhrase returns the phrase at idx, wrapping around the slice.
// Negative inputs are normalized to a positive index. Always returns a
// non-empty string so the indicator never flickers blank.
func thinkingPhrase(idx int) string {
	n := len(thinkingPhrases)
	if n == 0 {
		return "Thinking…"
	}
	i := idx % n
	if i < 0 {
		i += n
	}
	return thinkingPhrases[i]
}

// renderThinkingLine builds the in-chat thinking indicator. It pairs
// the spinner glyph (already brand-cyan from styles.Spinner via
// model.go wiring) with the rotating phrase, so the chat indicator
// reads as the same "system is working" affordance as the footer.
//
// The phrase is rendered with bold + brand cyan (no italic). VS Code's
// integrated terminal — and a handful of other xterm.js-based hosts —
// silently drop italic spans depending on the configured font, which
// produced reports of "I see the spinner but no text" in the wild.
// Bold + foreground color is the most portable visible styling.
func (m *Model) renderThinkingLine() string {
	phrase := thinkingPhrase(m.thinkingIdx)
	style := lipgloss.NewStyle().Foreground(brandCyan).Bold(true)
	return m.spinner.View() + " " + style.Render(phrase)
}
