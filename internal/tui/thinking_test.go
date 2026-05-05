// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/go-steer/cogo/internal/config"
)

// TestThinkingPhrase_WrapsAndAnchors pins two contracts at once:
//
//   - thinkingPhrase(0) MUST return the anchor phrase ("Thinking…")
//     so every turn begins with the unambiguous indicator before the
//     cheeky rotator wanders into the AI / sci-fi / CS jokes. Users
//     who don't get the joke still see "Thinking…" first.
//   - thinkingPhrase wraps cleanly past the end of the slice so the
//     rotator never panics on long-running turns.
//
// DO NOT silence this test if it breaks. A regression here either (a)
// removes the user-friendly anchor, leaving newcomers confused about
// what "Reticulating splines…" means, or (b) crashes the rotator on a
// long turn, which is exactly when users most need the indicator.
func TestThinkingPhrase_WrapsAndAnchors(t *testing.T) {
	t.Parallel()
	if got := thinkingPhrase(0); got != "Thinking…" {
		t.Errorf("thinkingPhrase(0) = %q, want anchor %q", got, "Thinking…")
	}
	n := len(thinkingPhrases)
	if got, want := thinkingPhrase(n), thinkingPhrases[0]; got != want {
		t.Errorf("thinkingPhrase(n) did not wrap to index 0; got %q want %q", got, want)
	}
	if got, want := thinkingPhrase(-1), thinkingPhrases[n-1]; got != want {
		t.Errorf("thinkingPhrase(-1) did not wrap to last index; got %q want %q", got, want)
	}
	if n < 10 {
		t.Errorf("thinkingPhrases has %d entries; the spec calls for 10-15 so the rotator stays interesting", n)
	}
}

// TestRenderMessage_StreamingPlaceholderShowsThinking pins the chat
// indicator contract: while the assistant placeholder has no text yet
// AND the model is in StateStreaming, the assistant slot in the chat
// must render the rotating thinking indicator (not a blank line).
//
// DO NOT silence this test if it breaks. A failure means the user
// sends a prompt and stares at an empty space until the first chunk
// streams in — the exact "is anything happening?" UX gap this feature
// closes. If the rendering path legitimately changes, replace the
// assertion with one that proves the new path still surfaces the
// indicator below the user's prompt; never delete the contract.
func TestRenderMessage_StreamingPlaceholderShowsThinking(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, "dark")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.history.Append(Message{Role: RoleUser, Text: "hello"})
	m.history.Append(Message{Role: RoleAssistant}) // empty placeholder
	m.currentAssistantIdx = 1
	m.state = StateStreaming
	m.thinkingIdx = 0
	m.refreshViewport()

	// View() output covers header + viewport + input + footer; the
	// footer ALSO says "Thinking…" in streaming mode, so we can't
	// just grep the whole frame for that token. Pull the viewport
	// region directly to assert the chat-window indicator.
	body := stripANSI(m.viewport.View())
	if !strings.Contains(body, "Thinking…") {
		t.Errorf("expected anchor phrase 'Thinking…' in viewport while streaming; got:\n%s", body)
	}

	// Rotate and verify the next phrase shows up after a refresh.
	m.thinkingIdx = 1
	m.refreshViewport()
	body = stripANSI(m.viewport.View())
	if !strings.Contains(body, thinkingPhrases[1]) {
		t.Errorf("expected rotated phrase %q in viewport; got:\n%s", thinkingPhrases[1], body)
	}

	// Once a chunk lands, the indicator must give way to the response.
	m.history.AppendText(1, "actual response text")
	m.refreshViewport()
	body = stripANSI(m.viewport.View())
	if strings.Contains(body, "Thinking…") || strings.Contains(body, thinkingPhrases[1]) {
		t.Errorf("thinking indicator should be hidden once the assistant message has content; got:\n%s", body)
	}
	if !strings.Contains(body, "actual response text") {
		t.Errorf("response text should be visible after first chunk; got:\n%s", body)
	}
}

// TestRenderThinkingLine_NoItalic pins the visibility contract for the
// in-chat indicator: the rotating phrase must NOT be styled with
// italic (SGR 3). VS Code's integrated terminal — among others —
// silently drops italic spans depending on the font, which surfaced as
// "I see the spinner but no text" in v0.1.2 dogfood. Bold + foreground
// color is the most portable visible styling.
//
// DO NOT silence this test if it breaks. Re-enabling italic on the
// indicator brings back a real bug that hides the affordance from a
// large chunk of the user base on dev-friendly terminal hosts.
func TestRenderThinkingLine_NoItalic(t *testing.T) {
	// Force truecolor so lipgloss actually emits ANSI; in the default
	// test profile lipgloss is a no-op and there'd be no SGR codes to
	// inspect, defeating the test. Not parallel because we mutate the
	// process-global lipgloss color profile.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, "dark")
	m.thinkingIdx = 0
	out := m.renderThinkingLine()
	// SGR 3 is the italic flag. Match either the standalone `\x1b[3m`
	// form or the parameterized `;3;` / `\x1b[3;` forms that lipgloss
	// emits when combining italic with foreground/bold.
	for _, marker := range []string{"\x1b[3m", "\x1b[3;", ";3;", ";3m"} {
		if strings.Contains(out, marker) {
			t.Errorf("renderThinkingLine() emitted italic SGR %q; would render invisible on VS Code/xterm.js terminals.\nraw: %q", marker, out)
		}
	}
	// Sanity: the phrase still has to be in there.
	if !strings.Contains(stripANSI(out), "Thinking…") {
		t.Errorf("renderThinkingLine() lost the phrase; got: %q", out)
	}
}

// TestRenderMessage_IdleAssistantNoThinking guards against the inverse
// regression: when the agent is idle (e.g. a stale assistant message
// whose render somehow gets re-evaluated), the thinking indicator must
// NOT appear. Otherwise users see "Thinking…" forever after a turn
// finishes — worse than no indicator at all.
func TestRenderMessage_IdleAssistantNoThinking(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, "dark")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.history.Append(Message{Role: RoleUser, Text: "hi"})
	m.history.Append(Message{Role: RoleAssistant}) // empty assistant
	m.state = StateIdle                            // not streaming
	m.refreshViewport()

	body := stripANSI(m.viewport.View())
	if strings.Contains(body, "Thinking…") {
		t.Errorf("thinking indicator should NOT appear in chat while idle; got:\n%s", body)
	}
}
