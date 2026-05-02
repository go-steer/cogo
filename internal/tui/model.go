// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/mcp"
	"github.com/go-steer/cogo/internal/memory"
	"github.com/go-steer/cogo/internal/permissions"
	"github.com/go-steer/cogo/internal/skills"
	"github.com/go-steer/cogo/internal/usage"
)

// State tracks the agent's current activity for input gating and View rendering.
type State int

const (
	StateIdle      State = iota // accepting input, no turn in flight
	StateStreaming              // a turn is running, input disabled
)

// Model is Cogo's Bubble Tea model. Mutated through *Model receivers so
// goroutine-driven Sends can update the same instance.
type Model struct {
	// Set by program.go after tea.NewProgram constructs the program.
	program programSender

	cfg   *config.Config
	agent *agent.Agent

	// UI components.
	history  History
	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model
	keys     KeyMap
	styles   Styles
	md       *MarkdownRenderer

	// Style name passed to Glamour. Resolved once at construction so
	// repeat renderer builds (on every resize) don't re-query the
	// terminal background.
	mdStyle string

	width  int
	height int

	// Turn state.
	state               State
	cancelTurn          context.CancelFunc
	currentAssistantIdx int // index in history of the in-progress assistant msg, -1 if none

	// Slash-command state.
	confirmingClear bool

	// Open palette overlay (slash-command discovery or @-file picker).
	// Non-nil while the overlay is visible; key handling intercepts
	// up/down/enter/esc in that case.
	palette *paletteState

	// projectRoot is the resolved directory used as the source for the
	// @-file picker; defaults to the cwd at NewModel time.
	projectRoot string

	// scope is consulted only to warn about @-file references that
	// point outside the in-scope roots. The user's keystroke is
	// authoritative consent (we still inline the file), but a system
	// message keeps them aware so they don't paste private files into
	// the model context by accident. Nil-safe.
	scope *permissions.PathScope

	// memory holds the AGENTS.md/CLAUDE.md/GEMINI.md contents loaded
	// at startup; surfaced via /memory.
	memory memory.Loaded

	// usage records per-turn token + cost accounting; surfaced via
	// /stats and the per-message footer + header running total.
	usage *usage.Tracker

	// rebuildAgent rebuilds the agent + runner with a new model ID,
	// preserving memory + tools. Set by program.go so /model can
	// switch mid-session without the TUI knowing about provider /
	// gate / tools wiring.
	rebuildAgent func(modelID string) (*agent.Agent, error)

	// reloadFromDisk re-reads .agents/ (mcp.json, skills/, AGENTS.md,
	// config.json) and rebuilds the agent in place. The new agent +
	// state get installed on the model. Set by program.go; nil-safe.
	reloadFromDisk func() (reloadResult, error)

	// persistModelChoice saves the new model choice to
	// .agents/config.json when invoked. May be nil if no project
	// config exists; in that case the switch is in-session only.
	persistModelChoice func(modelID string) error

	// modelPicker is the open Model picker overlay, if any.
	modelPicker *modelPickerState

	// mcpServers + skills carry the discovered extensibility for
	// /mcp + /skills rendering. Both are nil-safe.
	mcpServers []*mcp.Server
	skills     skills.Skills

	// Pending permission request from the gate. Non-nil while the
	// permission modal is up; the user's keypress writes back to
	// pendingConfirm.Out and clears this field.
	pendingConfirm *confirmReqMsg

	// Pending MCP elicitation request. Non-nil while the elicit modal
	// is up; key handling intercepts Tab / Enter / Esc / printable
	// keys until the user replies.
	pendingElicit *elicitState

	// Prompt history: the user's submitted prompts in submission
	// order. cursor is the active recall position when navigating
	// (-1 = not navigating, len(promptHistory) = past-end / empty input).
	promptHistory []string
	historyCursor int

	// True when the user just hit Ctrl+C while idle once. Second press exits.
	pendingExit bool

	// AlwaysAllow is invoked when the user picks "always allow" in the
	// permission modal. The host (TUI launcher) plugs in a function
	// that persists the pattern to .agents/config.json. May be nil in
	// tests.
	AlwaysAllow func(req permissions.PromptRequest) error
}

// NewModel constructs a fresh chat session bound to a configured agent.
// The program reference is set later via SetProgram.
//
// mdStyle picks the Glamour style ("dark" or "light") for assistant
// markdown rendering. Detect this once before tea.NewProgram (see
// program.Run); resolving it during the program's lifetime causes
// Glamour's background-color query response to leak into the textarea.
func NewModel(cfg *config.Config, a *agent.Agent, mdStyle string) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message, or /help…"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.Focus()

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	md, _ := NewMarkdownRenderer(80, mdStyle) // tightened on first WindowSizeMsg

	cwd, _ := os.Getwd()

	m := &Model{
		cfg:                 cfg,
		agent:               a,
		textarea:            ta,
		viewport:            vp,
		spinner:             sp,
		keys:                DefaultKeyMap(),
		styles:              DefaultStyles(),
		md:                  md,
		mdStyle:             mdStyle,
		state:               StateIdle,
		currentAssistantIdx: -1,
		projectRoot:         cwd,
		historyCursor:       -1,
		usage:               usage.NewTracker(),
	}
	return m
}

// SetProgram wires the running tea.Program in so background goroutines
// (the agent runner) can Send messages back to the loop.
func (m *Model) SetProgram(p programSender) { m.program = p }

// Init returns the initial commands. The spinner is started so its Tick
// loop can animate when transitioning into the streaming state; the
// textarea blink keeps the cursor visible.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

// renderHistory builds the viewport contents from the current history.
// One block per message, separated by blank lines.
func (m *Model) renderHistory() string {
	if m.history.Len() == 0 {
		return m.styles.System.Render("Type a message and hit Enter. /help for commands, /quit to exit.")
	}
	msgs := m.history.Snapshot()
	parts := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		parts = append(parts, m.renderMessage(msg))
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) renderMessage(msg Message) string {
	switch msg.Role {
	case RoleUser:
		prefix := m.styles.UserPrefix.Render("❯")
		return prefix + " " + m.styles.UserText.Render(msg.Display())
	case RoleAssistant:
		// Display() prefers the Glamour-rendered form when available;
		// during streaming it falls back to raw text.
		text := msg.Display()
		if msg.Rendered == "" {
			// Streaming: render raw with the assistant style for color.
			return m.styles.Assistant.Render(text)
		}
		// Append a per-prompt usage footer when available.
		if footer := m.lastTurnUsageFooter(); footer != "" {
			return text + "\n" + footer
		}
		return text
	case RoleSystem:
		return m.styles.System.Render(msg.Display())
	case RoleError:
		return m.styles.Error.Render(msg.Display())
	default:
		return msg.Display()
	}
}

// lastTurnUsageFooter renders the most recent turn's usage as a small
// muted footer line. Empty when no tracker is wired or no turns yet.
func (m *Model) lastTurnUsageFooter() string {
	if m.usage == nil {
		return ""
	}
	last, ok := m.usage.Last()
	if !ok {
		return ""
	}
	line := fmt.Sprintf("↑%d in · ↓%d out · $%s", last.InputTokens, last.OutputTokens, formatCost(last.CostUSD))
	return m.styles.Footer.Render(line)
}

// refreshViewport re-renders the history into the viewport. If the user
// was already pinned to the bottom (the common "tail" position), scroll
// stays at the bottom as new content arrives. If they had scrolled up
// to read history, leave them where they were.
func (m *Model) refreshViewport() {
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(m.renderHistory())
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// reloadResult is what reloadFromDisk hands back to the model so the
// fresh state can be installed in one atomic update.
type reloadResult struct {
	Agent      *agent.Agent
	Memory     memory.Loaded
	MCPServers []*mcp.Server
	Skills     skills.Skills
}

// renderMemoryInfo formats the loaded-memory provenance for /memory.
func (m *Model) renderMemoryInfo() string {
	if m.memory.Empty() {
		return "No memory loaded.\n\nDrop AGENTS.md, CLAUDE.md, or GEMINI.md at the project root, or ~/.cogo/AGENTS.md for personal preferences."
	}
	var b strings.Builder
	b.WriteString("Memory loaded:\n")
	for _, s := range m.memory.Sources {
		marker := ""
		if s.Truncated {
			marker = " (truncated)"
		}
		b.WriteString("  ")
		b.WriteString(s.Scope)
		b.WriteString(": ")
		b.WriteString(s.Path)
		b.WriteString(" — ")
		b.WriteString(formatBytes(s.Bytes))
		b.WriteString(marker)
		b.WriteByte('\n')
	}
	return b.String()
}

// renderMCPInfo formats the configured MCP servers for /mcp.
// Each server is followed by an indented list of the tools it
// exposes (the namespaced names the agent actually sees).
func (m *Model) renderMCPInfo() string {
	if len(m.mcpServers) == 0 {
		return "No MCP servers configured. Drop a .agents/mcp.json describing servers (stdio or HTTP transport) to expose external tools to the agent."
	}
	var b strings.Builder
	b.WriteString("MCP servers:\n")
	for _, s := range m.mcpServers {
		b.WriteString("  ")
		b.WriteString(s.Name)
		b.WriteString(" — ")
		b.WriteString(s.Status)
		if s.Err != nil {
			b.WriteString(" (")
			b.WriteString(s.Err.Error())
			b.WriteString(")")
		}
		b.WriteByte('\n')
		switch {
		case s.Status != "ok":
			// Skip tool list for failed servers.
		case len(s.Tools) == 0:
			b.WriteString("      (server exposes no tools, or enumeration failed)\n")
		default:
			for _, t := range s.Tools {
				b.WriteString("      • ")
				b.WriteString(t)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// renderSkillsInfo formats the discovered skills for /skills.
func (m *Model) renderSkillsInfo() string {
	if m.skills.Empty() {
		return "No skills discovered. Drop SKILL.md bundles under .agents/skills/<name>/ to expose them to the agent."
	}
	var b strings.Builder
	b.WriteString("Skills:\n")
	for _, info := range m.skills.Infos {
		b.WriteString("  ")
		b.WriteString(info.Name)
		if info.Description != "" {
			b.WriteString(" — ")
			b.WriteString(info.Description)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// renderStatsInfo formats the per-turn + total usage for /stats.
func (m *Model) renderStatsInfo() string {
	if m.usage == nil {
		return "Usage tracking not available."
	}
	tot := m.usage.Totals()
	if tot.Turns == 0 {
		return "No turns yet — try sending a prompt first."
	}
	var b strings.Builder
	b.WriteString("Session stats:\n")
	b.WriteString("  Turns:    ")
	b.WriteString(strconv.Itoa(tot.Turns))
	b.WriteByte('\n')
	b.WriteString("  Input:    ")
	b.WriteString(strconv.Itoa(tot.InputTokens))
	b.WriteString(" tokens\n")
	b.WriteString("  Output:   ")
	b.WriteString(strconv.Itoa(tot.OutputTokens))
	b.WriteString(" tokens\n")
	b.WriteString("  Cost:     $")
	b.WriteString(formatCost(tot.CostUSD))
	b.WriteByte('\n')
	b.WriteString("  Duration: ")
	b.WriteString(m.usage.Duration().Round(0).String())
	b.WriteByte('\n')
	b.WriteString("  Model:    ")
	b.WriteString(m.cfg.Model.Name)
	return b.String()
}

func formatBytes(n int) string {
	if n >= 1024 {
		return fmt.Sprintf("%d KiB", n/1024)
	}
	return fmt.Sprintf("%d B", n)
}

// formatCost renders c with 4 decimals, trimming trailing zeros so
// "$0.0019" stays compact and "$0.1500" becomes "$0.15".
func formatCost(c float64) string {
	s := fmt.Sprintf("%.4f", c)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}
