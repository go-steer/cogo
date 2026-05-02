package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update is Cogo's central message dispatch.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case streamChunkMsg:
		return m.handleStreamChunk(msg)
	case turnDoneMsg:
		return m.handleTurnDone()
	case turnErrMsg:
		return m.handleTurnErr(msg)
	case turnCancelledMsg:
		return m.handleTurnCancelled()
	case spinner.TickMsg:
		// Only animate while streaming to avoid background CPU usage.
		if m.state != StateStreaming {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	// Unhandled — forward typing/etc. to the textarea.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	headerH := 1
	inputH := m.textarea.Height() + 2 // border lines
	footerH := 1
	vpH := m.height - headerH - inputH - footerH
	if vpH < 3 {
		vpH = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpH
	m.textarea.SetWidth(m.width - 4) // border + padding
	// Re-init markdown renderer at the new wrap width. Using the
	// pre-resolved style name avoids re-querying the terminal.
	if md, err := NewMarkdownRenderer(m.width-2, m.mdStyle); err == nil {
		m.md = md
	}
	m.refreshViewport()
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.handleCtrlC()
	case key.Matches(msg, m.keys.ClearView):
		m.viewport.GotoTop()
		return m, nil
	case key.Matches(msg, m.keys.ScrollUp), key.Matches(msg, m.keys.ScrollDown):
		// PgUp/PgDn always scroll the viewport.
		m.pendingExit = false
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case key.Matches(msg, m.keys.LineUp), key.Matches(msg, m.keys.LineDown):
		// Up/Down scroll the viewport line-by-line — but only when the
		// input is empty, so multi-line cursor navigation still works
		// normally while composing a message.
		if m.textarea.Value() == "" {
			m.pendingExit = false
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case key.Matches(msg, m.keys.Submit):
		// Submit Enter only fires a turn when idle. While streaming we
		// swallow it so users don't accidentally enqueue a half-composed
		// prompt; typed text continues to land in the textarea below so
		// they can compose their next message in the background.
		if m.state == StateStreaming {
			return m, nil
		}
		return m.handleSubmit()
	}
	// Reset pendingExit on any other key so a stray Ctrl+C doesn't linger.
	m.pendingExit = false

	// Always forward character/navigation keys to the textarea — even
	// during streaming — so the user's input doesn't disappear into a
	// state-machine race when the turn ends mid-typing.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *Model) handleCtrlC() (tea.Model, tea.Cmd) {
	if m.state == StateStreaming {
		// Cancel current turn.
		if m.cancelTurn != nil {
			m.cancelTurn()
		}
		return m, nil
	}
	// Idle: first press warns, second exits.
	if !m.pendingExit {
		m.pendingExit = true
		m.history.Append(Message{Role: RoleSystem, Text: "Press Ctrl+C again to exit, or any key to cancel."})
		m.refreshViewport()
		return m, nil
	}
	return m, tea.Quit
}

func (m *Model) handleSubmit() (tea.Model, tea.Cmd) {
	input := m.textarea.Value()
	if strings.TrimSpace(input) == "" {
		return m, nil
	}

	// Confirmation flow for /clear.
	if m.confirmingClear {
		m.confirmingClear = false
		m.textarea.Reset()
		if isYes(input) {
			m.history.Reset()
			m.history.Append(Message{Role: RoleSystem, Text: "History cleared."})
		} else {
			m.history.Append(Message{Role: RoleSystem, Text: "Cancelled."})
		}
		m.refreshViewport()
		return m, nil
	}

	// Slash command?
	if action, cmd, isSlash := ParseSlash(input); isSlash {
		m.textarea.Reset()
		return m.handleSlash(action, cmd)
	}

	// Regular prompt → start a turn.
	m.history.Append(Message{Role: RoleUser, Text: input})
	m.textarea.Reset()
	idx := m.history.Append(Message{Role: RoleAssistant})
	m.currentAssistantIdx = idx
	m.state = StateStreaming

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelTurn = cancel
	startAgentTurn(ctx, m.program, m.agent, input)

	m.refreshViewport()
	return m, m.spinner.Tick
}

func (m *Model) handleSlash(action SlashAction, cmd string) (tea.Model, tea.Cmd) {
	switch action {
	case SlashHelp:
		m.history.Append(Message{Role: RoleSystem, Text: HelpText()})
		m.refreshViewport()
		return m, nil
	case SlashClear:
		m.confirmingClear = true
		m.history.Append(Message{Role: RoleSystem, Text: "Clear chat history? Type 'y' or 'yes' to confirm; anything else cancels."})
		m.refreshViewport()
		return m, nil
	case SlashQuit:
		return m, tea.Quit
	default:
		m.history.Append(Message{Role: RoleSystem, Text: fmt.Sprintf("Unknown command: /%s. Type /help for the list.", cmd)})
		m.refreshViewport()
		return m, nil
	}
}

func (m *Model) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	if m.currentAssistantIdx < 0 {
		return m, nil
	}
	m.history.AppendText(m.currentAssistantIdx, msg.Text)
	m.refreshViewport()
	return m, nil
}

func (m *Model) handleTurnDone() (tea.Model, tea.Cmd) {
	if m.currentAssistantIdx >= 0 {
		// Re-render the completed assistant message through Glamour.
		raw := m.history.Snapshot()[m.currentAssistantIdx].Text
		m.history.SetRendered(m.currentAssistantIdx, strings.TrimRight(m.md.Render(raw), "\n"))
	}
	m.endTurn()
	m.refreshViewport()
	return m, nil
}

func (m *Model) handleTurnErr(msg turnErrMsg) (tea.Model, tea.Cmd) {
	if m.currentAssistantIdx >= 0 {
		// If we accumulated any partial output, leave it; just append an
		// error notice afterward.
		current := m.history.Snapshot()[m.currentAssistantIdx]
		if current.Text == "" {
			// Drop the empty assistant placeholder rather than rendering a blank slot.
			m.dropLastAssistant()
		}
	}
	m.history.Append(Message{Role: RoleError, Text: fmt.Sprintf("Error: %v", msg.Err)})
	m.endTurn()
	m.refreshViewport()
	return m, nil
}

func (m *Model) handleTurnCancelled() (tea.Model, tea.Cmd) {
	if m.currentAssistantIdx >= 0 {
		current := m.history.Snapshot()[m.currentAssistantIdx]
		if current.Text == "" {
			m.dropLastAssistant()
		}
	}
	m.history.Append(Message{Role: RoleSystem, Text: "(interrupted)"})
	m.endTurn()
	m.refreshViewport()
	return m, nil
}

func (m *Model) endTurn() {
	m.state = StateIdle
	m.currentAssistantIdx = -1
	if m.cancelTurn != nil {
		m.cancelTurn()
		m.cancelTurn = nil
	}
}

// dropLastAssistant rewinds the in-progress assistant message. Called
// when the turn ended before any text was produced.
func (m *Model) dropLastAssistant() {
	if m.currentAssistantIdx < 0 {
		return
	}
	snap := m.history.Snapshot()
	m.history.Reset()
	for i, msg := range snap {
		if i == m.currentAssistantIdx {
			continue
		}
		m.history.Append(msg)
	}
}

func isYes(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return t == "y" || t == "yes"
}
