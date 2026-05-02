package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-steer/cogo/internal/permissions"
)

// Update is Cogo's central message dispatch.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		// Permission modal preempts every other key handler when up.
		if m.pendingConfirm != nil {
			return m.handleConfirmKey(msg)
		}
		return m.handleKey(msg)
	case confirmReqMsg:
		// Show modal; remember the request so handleConfirmKey can
		// reply to the same channel. If a request is already in flight
		// we deny the new one immediately to avoid stacking.
		if m.pendingConfirm != nil {
			msg.Out <- permissions.DecisionDeny
			return m, nil
		}
		m.pendingConfirm = &msg
		return m, nil
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

// handleConfirmKey resolves the pending permission request based on the
// user's keypress. Anything other than the four configured keys is
// ignored so accidental typing doesn't auto-deny.
func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingConfirm == nil {
		return m, nil
	}
	var d permissions.Decision
	switch {
	case key.Matches(msg, m.keys.ConfirmAllowOnce):
		d = permissions.DecisionAllowOnce
	case key.Matches(msg, m.keys.ConfirmAllowSession):
		d = permissions.DecisionAllowSession
	case key.Matches(msg, m.keys.ConfirmAllowAlways):
		d = permissions.DecisionAllowAlways
	case key.Matches(msg, m.keys.ConfirmDeny):
		d = permissions.DecisionDeny
	default:
		return m, nil
	}
	req := m.pendingConfirm.Req
	m.pendingConfirm.Out <- d
	m.pendingConfirm = nil

	// Echo the user's choice into the chat so there's a paper trail.
	m.history.Append(Message{Role: RoleSystem, Text: confirmEcho(req, d)})

	// "Always allow" persists via the host-supplied callback.
	if d == permissions.DecisionAllowAlways && m.AlwaysAllow != nil {
		if err := m.AlwaysAllow(req); err != nil {
			m.history.Append(Message{Role: RoleError, Text: "Couldn't persist allowlist entry: " + err.Error()})
		}
	}
	m.refreshViewport()
	return m, nil
}

func confirmEcho(req permissions.PromptRequest, d permissions.Decision) string {
	return "Permission " + d.String() + ": " + req.ToolName + " — " + req.Detail
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
	// Palette intercepts up/down/enter/esc/tab when open.
	if m.palette != nil {
		switch msg.String() {
		case "up":
			if m.palette.cursor > 0 {
				m.palette.cursor--
			}
			return m, nil
		case "down":
			if m.palette.cursor < len(m.palette.items)-1 {
				m.palette.cursor++
			}
			return m, nil
		case "esc":
			m.palette = nil
			return m, nil
		case "tab":
			// Tab fills the highlighted item without submitting (slash
			// commands stay un-submitted so the user can add args).
			return m.applyPaletteCompletion()
		case "enter":
			return m.applyPaletteSelection()
		}
		// Other keys fall through to the textarea (typing filters the palette).
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		return m.handleCtrlC()
	case key.Matches(msg, m.keys.ClearView):
		m.viewport.GotoTop()
		return m, nil
	case key.Matches(msg, m.keys.ClearInput):
		m.textarea.Reset()
		m.historyCursor = -1
		m.refreshPalette()
		return m, nil
	case key.Matches(msg, m.keys.ScrollUp), key.Matches(msg, m.keys.ScrollDown):
		// PgUp/PgDn always scroll the viewport.
		m.pendingExit = false
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case key.Matches(msg, m.keys.LineUp):
		// Up on empty input: recall previous prompts (shell-style
		// history). When already navigating, step further back.
		// Otherwise (input has text) the keypress falls through to the
		// textarea for cursor movement.
		if m.textarea.Value() == "" || m.historyCursor >= 0 {
			m.recallPrompt(-1)
			return m, nil
		}
	case key.Matches(msg, m.keys.LineDown):
		// Down: step forward through history when navigating; otherwise
		// fall through to textarea cursor movement (most common while
		// composing).
		if m.historyCursor >= 0 {
			m.recallPrompt(+1)
			return m, nil
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

	// After the textarea consumes the key, re-evaluate whether a
	// palette should be open or closed.
	m.refreshPalette()
	return m, cmd
}

// refreshPalette syncs m.palette with the textarea state. Called after
// any keystroke that may have changed the input.
func (m *Model) refreshPalette() {
	value := m.textarea.Value()
	cursor := len(value) // bubbles textarea uses byte offsets; cursor approximated as end
	kind, triggerPos, filter, ok := detectPaletteTrigger(value, cursor)
	if !ok {
		m.palette = nil
		return
	}
	var items []paletteItem
	switch kind {
	case paletteSlash:
		items = filterPaletteItems(allSlashItems(), filter)
	case paletteFile:
		items = listProjectFiles(m.projectRoot, filter)
	}
	if len(items) == 0 {
		m.palette = nil
		return
	}
	cur := 0
	if m.palette != nil && m.palette.kind == kind {
		// Preserve cursor if still in range; otherwise clamp.
		cur = m.palette.cursor
		if cur >= len(items) {
			cur = len(items) - 1
		}
		if cur < 0 {
			cur = 0
		}
	}
	m.palette = &paletteState{
		kind:       kind,
		items:      items,
		cursor:     cur,
		triggerPos: triggerPos,
		filter:     filter,
	}
	if kind == paletteSlash {
		m.palette.trigger = '/'
	} else {
		m.palette.trigger = '@'
	}
}

// recallPrompt steps the history cursor by delta and updates the
// textarea. delta is -1 for "older" and +1 for "newer". The cursor's
// final position is clamped to [-1, len(promptHistory)]; reaching
// past-end clears the input and exits navigation mode.
func (m *Model) recallPrompt(delta int) {
	if len(m.promptHistory) == 0 {
		return
	}
	switch {
	case m.historyCursor < 0:
		// Begin navigation from the most recent.
		m.historyCursor = len(m.promptHistory) - 1
	default:
		m.historyCursor += delta
	}
	switch {
	case m.historyCursor < 0:
		m.historyCursor = 0
	case m.historyCursor >= len(m.promptHistory):
		// Past end → clear input and exit navigation.
		m.historyCursor = -1
		m.textarea.SetValue("")
		m.refreshPalette()
		return
	}
	m.textarea.SetValue(m.promptHistory[m.historyCursor])
	m.refreshPalette()
}

// applyPaletteSelection acts on Enter while the palette is open. Slash
// items: replace the input with the selected command and submit
// immediately. File items: insert the @-path at the trigger position;
// directories drill in (palette stays open with the new filter), files
// finalize and close the palette.
func (m *Model) applyPaletteSelection() (tea.Model, tea.Cmd) {
	if m.palette == nil || len(m.palette.items) == 0 {
		return m, nil
	}
	sel := m.palette.items[m.palette.cursor]
	switch m.palette.kind {
	case paletteSlash:
		m.palette = nil
		m.textarea.SetValue(sel.Value)
		return m.handleSubmit()
	case paletteFile:
		current := m.textarea.Value()
		// Drilling into a dir: replace the partial @-token with the
		// directory's value (which ends in "/") and let refreshPalette
		// re-list files filtered by the new path.
		if sel.IsDir {
			newVal := current[:m.palette.triggerPos] + sel.Value
			m.textarea.SetValue(newVal)
			m.refreshPalette()
			return m, nil
		}
		// File: insert + space + close palette.
		newVal := current[:m.palette.triggerPos] + sel.Value + " "
		m.textarea.SetValue(newVal)
		m.palette = nil
		return m, nil
	}
	return m, nil
}

// applyPaletteCompletion is the Tab variant: like Enter for files, but
// for slash commands it inserts "<command> " (with trailing space) and
// closes the palette without submitting, so the user can add args.
func (m *Model) applyPaletteCompletion() (tea.Model, tea.Cmd) {
	if m.palette == nil || len(m.palette.items) == 0 {
		return m, nil
	}
	sel := m.palette.items[m.palette.cursor]
	switch m.palette.kind {
	case paletteSlash:
		m.textarea.SetValue(sel.Value + " ")
		m.palette = nil
		return m, nil
	case paletteFile:
		// Same as Enter for files (drill-in for dirs; insert+close for files).
		return m.applyPaletteSelection()
	}
	return m, nil
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

	// Regular prompt → start a turn. Expand any @<path> file references
	// before sending to the model; show the user-facing prompt as-typed
	// in history (preserving the @ tokens) but pass the expanded form
	// to the agent so it has the file contents inline.
	m.history.Append(Message{Role: RoleUser, Text: input})
	// Recall history: append the submitted prompt and reset the cursor.
	m.promptHistory = append(m.promptHistory, input)
	m.historyCursor = -1
	expanded, refs, diags := expandAtRefs(input, readFileSafe(64*1024))
	for _, d := range diags {
		m.history.Append(Message{Role: RoleSystem, Text: d})
	}
	if len(refs) > 0 {
		// Surface a warning for any @-ref that lands outside the
		// configured path scope. We still inlined the file (the user
		// typed the @-token explicitly) but they should be aware.
		var outOfScope []string
		if m.scope != nil {
			for _, r := range refs {
				if in, _ := m.scope.Contains(r); !in {
					outOfScope = append(outOfScope, r)
				}
			}
		}
		if len(outOfScope) > 0 {
			m.history.Append(Message{
				Role: RoleSystem,
				Text: "⚠ Inlined out-of-scope file(s): " + strings.Join(outOfScope, ", ") +
					" — these were sent to the model. Add them to .agents/config.json path_scope.allow if you want this without the warning.",
			})
		}
		m.history.Append(Message{Role: RoleSystem, Text: "Inlined file references: " + strings.Join(refs, ", ")})
	}
	m.textarea.Reset()
	m.palette = nil
	idx := m.history.Append(Message{Role: RoleAssistant})
	m.currentAssistantIdx = idx
	m.state = StateStreaming

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelTurn = cancel
	startAgentTurn(ctx, m.program, m.agent, expanded)

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
