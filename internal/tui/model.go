package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
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

	// True when the user just hit Ctrl+C while idle once. Second press exits.
	pendingExit bool
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
		return text
	case RoleSystem:
		return m.styles.System.Render(msg.Display())
	case RoleError:
		return m.styles.Error.Render(msg.Display())
	default:
		return msg.Display()
	}
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
