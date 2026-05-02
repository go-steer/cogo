package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the model as a single string. Layout (top to bottom):
//
//	Header
//	Viewport (scrollable history)
//	Input area (textarea inside a rounded border) — replaced by the
//	  permission modal when a request is pending
//	Footer (status hint or spinner)
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		// Pre-resize: avoid drawing into 0×0.
		return "Loading…"
	}

	header := m.renderHeader()
	body := m.viewport.View()
	var input string
	if m.pendingConfirm != nil {
		input = m.renderConfirmModal()
	} else {
		input = m.renderInput()
	}
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, input, footer)
}

// renderConfirmModal draws the permission request modal in place of
// the input area. Kept simple in Slice 3: a bordered box with the
// request detail and the four-key prompt.
func (m *Model) renderConfirmModal() string {
	req := m.pendingConfirm.Req
	kindLabel := map[int]string{
		0: "Bash command",
		1: "File write",
		2: "Path scope",
		3: "Tool",
	}[int(req.Kind)]
	if kindLabel == "" {
		kindLabel = "Tool"
	}
	body := m.styles.Confirm.Render(kindLabel+": "+req.Detail) + "\n" +
		m.styles.Footer.Render("[y] allow once   [s] allow session   [a] always allow   [n/esc] deny")
	return m.styles.InputBorder.Render(body)
}

func (m *Model) renderHeader() string {
	provider := m.cfg.Model.Provider
	if provider == "" {
		provider = "auto"
	}
	left := fmt.Sprintf("Cogo · %s", m.styles.HeaderAccent.Render(m.cfg.Model.Name))
	right := fmt.Sprintf("provider: %s", provider)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.styles.Header.Render(left + strings.Repeat(" ", gap) + right)
}

func (m *Model) renderInput() string {
	return m.styles.InputBorder.Render(m.textarea.View())
}

func (m *Model) renderFooter() string {
	switch {
	case m.pendingConfirm != nil:
		return m.styles.Footer.Render("Permission required — choose one of the keys above")
	case m.state == StateStreaming:
		return m.styles.Footer.Render(m.spinner.View() + " Thinking… (Ctrl+C to cancel)")
	case m.confirmingClear:
		return m.styles.Confirm.Render("Confirm clear: type y / yes / anything else")
	default:
		return m.styles.Footer.Render("/help · /quit · Ctrl+C to exit")
	}
}
