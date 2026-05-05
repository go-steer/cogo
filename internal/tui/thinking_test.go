// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
