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
	if m.pendingConfirm != nil {
		input = m.renderConfirmModal()
	} else {
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
	right := fmt.Sprintf("%s · provider: %s · mode: %s", cwd, provider, modeBadge(mode, m.styles))
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
