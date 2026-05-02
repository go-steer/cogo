package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// View renders the model as a single string. Layout (top to bottom):
//
//	Header
//	Viewport (scrollable history)
//	Palette (slash / file picker — only when active)
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
	switch {
	case m.pendingElicit != nil:
		input = m.renderElicitModal()
	case m.pendingConfirm != nil:
		input = m.renderConfirmModal()
	case m.modelPicker != nil:
		input = m.renderModelPicker()
	default:
		input = m.renderInput()
	}
	footer := m.renderFooter()

	parts := []string{header, body}
	if m.palette != nil {
		parts = append(parts, m.renderPalette())
	}
	parts = append(parts, input, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderPalette draws the palette overlay between the viewport and the
// input area. Cursor row is highlighted; non-cursor rows are muted.
func (m *Model) renderPalette() string {
	p := m.palette
	rows := len(p.items)
	if rows > MaxPaletteRows {
		rows = MaxPaletteRows
	}
	// Window items so the cursor stays in view.
	start := 0
	if p.cursor >= rows {
		start = p.cursor - rows + 1
	}
	end := start + rows
	if end > len(p.items) {
		end = len(p.items)
	}

	var lines []string
	for i := start; i < end; i++ {
		it := p.items[i]
		marker := "  "
		if i == p.cursor {
			marker = "▸ "
		}
		line := marker + it.Display
		if it.Hint != "" {
			line += "  " + it.Hint
		}
		if i == p.cursor {
			lines = append(lines, m.styles.HeaderAccent.Render(line))
		} else {
			lines = append(lines, m.styles.Footer.Render(line))
		}
	}
	header := "  " + paletteHeader(p)
	body := strings.Join(lines, "\n")
	return m.styles.InputBorder.Render(m.styles.System.Render(header) + "\n" + body)
}

func paletteHeader(p *paletteState) string {
	switch p.kind {
	case paletteSlash:
		return "Slash commands (↑/↓ select · enter run · esc cancel)"
	case paletteFile:
		return "Files (↑/↓ select · enter insert · esc cancel)"
	default:
		return ""
	}
}

// renderModelPicker draws the /model picker in place of the input.
// Same shape as the slash/file palette but with model IDs.
func (m *Model) renderModelPicker() string {
	p := m.modelPicker
	header := m.styles.System.Render("Model picker (↑/↓ select · enter switch · esc cancel)")
	var lines []string
	for i, id := range p.items {
		marker := "  "
		if i == p.cursor {
			marker = "▸ "
		}
		line := marker + id
		if id == m.cfg.Model.Name {
			line += "  (current)"
		}
		if i == p.cursor {
			lines = append(lines, m.styles.HeaderAccent.Render(line))
		} else {
			lines = append(lines, m.styles.Footer.Render(line))
		}
	}
	body := strings.Join(lines, "\n")
	return m.styles.InputBorder.Render(header + "\n" + body)
}

// renderElicitModal draws the MCP elicitation modal. URL mode shows
// the URL prominently with open/accept/decline keys. Form mode lists
// each field with the active one highlighted, plus a key legend and
// any validation error.
func (m *Model) renderElicitModal() string {
	st := m.pendingElicit
	if st == nil {
		return ""
	}
	header := m.styles.Confirm.Render(fmt.Sprintf("MCP %s — input requested", st.ServerName))
	if st.Mode == elicitURL {
		body := header + "\n"
		if st.Message != "" {
			body += m.styles.System.Render(st.Message) + "\n"
		}
		body += m.styles.HeaderAccent.Render(st.URL) + "\n" +
			m.styles.Footer.Render("[o] open in browser   [a/enter] accept   [n] decline   [esc] cancel")
		return m.styles.InputBorder.Render(body)
	}

	lines := []string{header}
	if st.Message != "" {
		lines = append(lines, m.styles.System.Render(st.Message))
	}
	for i, f := range st.Fields {
		marker := "  "
		if i == st.Active {
			marker = "▸ "
		}
		label := marker + f.Name
		if f.Required {
			label += " *"
		}
		if f.Description != "" {
			label += "  " + m.styles.Footer.Render("("+f.Description+")")
		}
		var value string
		switch f.Kind {
		case fieldString, fieldNumber, fieldInteger:
			value = f.input.View()
		case fieldEnum, fieldBoolean:
			value = renderChoiceCycler(f, i == st.Active, m.styles)
		}
		row := label + "\n    " + value
		if i == st.Active {
			lines = append(lines, m.styles.HeaderAccent.Render(row))
		} else {
			lines = append(lines, m.styles.Footer.Render(row))
		}
	}
	if st.Err != "" {
		lines = append(lines, m.styles.Error.Render("⚠ "+st.Err))
	}
	lines = append(lines, m.styles.Footer.Render(
		"[tab/↓] next   [shift+tab/↑] prev   [enter] submit   [esc] cancel"))
	return m.styles.InputBorder.Render(strings.Join(lines, "\n"))
}

// renderChoiceCycler shows the enum/boolean choice as the current
// value flanked by '<' and '>' to hint at left/right cycling.
func renderChoiceCycler(f elicitField, active bool, st Styles) string {
	if len(f.Choices) == 0 {
		return "(no choices)"
	}
	cur := f.choice
	if cur < 0 || cur >= len(f.Choices) {
		cur = 0
	}
	val := f.Choices[cur]
	if active {
		return st.HeaderAccent.Render("‹ " + val + " ›")
	}
	return val
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
	mode := m.cfg.Permissions.Mode
	if mode == "" {
		mode = "ask"
	}
	cwd := shortDir(m.projectRoot)

	left := fmt.Sprintf("Cogo · %s", m.styles.HeaderAccent.Render(m.cfg.Model.Name))
	right := fmt.Sprintf("%s · %s · %s", cwd, provider, modeBadge(mode, m.styles))
	if m.usage != nil {
		tot := m.usage.Totals()
		right += fmt.Sprintf(" · σ ↑%d/↓%d/$%s",
			tot.InputTokens, tot.OutputTokens, formatCost(tot.CostUSD))
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return m.styles.Header.Render(left + strings.Repeat(" ", gap) + right)
}

// shortDir returns the basename of dir prefixed with "~/" when dir is
// inside the user's home; otherwise it falls back to the absolute
// basename. Empty dir → "?".
func shortDir(dir string) string {
	if dir == "" {
		return "?"
	}
	home := homeDir()
	if home != "" && (dir == home || strings.HasPrefix(dir, home+"/")) {
		rel := strings.TrimPrefix(dir, home)
		return "~" + rel
	}
	return dir
}

// modeBadge styles the permission mode so "yolo" stands out — landing
// in yolo without realizing it should be visually obvious.
func modeBadge(mode string, st Styles) string {
	switch mode {
	case "yolo":
		return st.Error.Render(mode)
	case "ask":
		return st.HeaderAccent.Render(mode)
	default:
		return mode
	}
}

func (m *Model) renderInput() string {
	return m.styles.InputBorder.Render(m.textarea.View())
}

func (m *Model) renderFooter() string {
	switch {
	case m.pendingElicit != nil:
		return m.styles.Footer.Render("MCP elicitation in progress — see the modal above")
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
