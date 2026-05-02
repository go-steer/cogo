// Package agent wraps the Google ADK runner with Cogo-specific defaults
// (streaming mode, in-memory session service, app name) so callers in
// cmd/cogo, internal/headless, and internal/tui all hit the same shape.
//
// Slice 1 supports a single-turn text agent with no tools. Tools, MCP
// toolsets, skills, and the permission gate land in later slices and
// will plug in here as Options.
package agent

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
)

// AppName tags this process in the ADK runner. Telemetry and session
// stores key off this; keep it stable.
const AppName = "cogo"

// defaultUserID and defaultSessionID are placeholders for Slice 1 where
// each invocation is a fresh single turn. Multi-turn TUI usage in Slice 2
// will replace these with per-session UUIDs.
const (
	defaultUserID    = "local"
	defaultSessionID = "default"
)

// Agent is Cogo's wrapper around an ADK llmagent + runner. One Agent
// represents one configured LLM-driven role; callers build one per
// session (or per process in the headless case).
type Agent struct {
	inner     adkagent.Agent
	runner    *runner.Runner
	streaming adkagent.StreamingMode
	userID    string
	sessionID string
}

// Option mutates Agent construction. Use the With* helpers below.
type Option func(*options)

type options struct {
	name        string
	description string
	instruction string
	streaming   adkagent.StreamingMode
	userID      string
	sessionID   string
	tools       []tool.Tool
}

func defaultOptions() options {
	return options{
		name:        "cogo_agent",
		description: "Cogo conversational agent",
		instruction: "You are Cogo, a terminal-based coding assistant. Be concise and accurate.",
		streaming:   adkagent.StreamingModeSSE,
		userID:      defaultUserID,
		sessionID:   defaultSessionID,
	}
}

// WithName overrides the agent's display name (visible in OTEL spans).
func WithName(s string) Option { return func(o *options) { o.name = s } }

// WithInstruction overrides the system instruction.
func WithInstruction(s string) Option { return func(o *options) { o.instruction = s } }

// WithStreaming overrides the streaming mode. Default is StreamingModeSSE
// (the spike confirmed this is required to receive Partial events).
func WithStreaming(m adkagent.StreamingMode) Option {
	return func(o *options) { o.streaming = m }
}

// WithSession overrides the user/session IDs handed to the ADK runner.
// Slice 1 callers don't need this; Slice 2's TUI will use it.
func WithSession(userID, sessionID string) Option {
	return func(o *options) { o.userID = userID; o.sessionID = sessionID }
}

// WithTools registers a set of tools the agent may call. Order is
// preserved but immaterial; ADK keys tools by Name.
func WithTools(ts []tool.Tool) Option {
	return func(o *options) { o.tools = append(o.tools, ts...) }
}

// New constructs an Agent backed by model. Returns a clear error if the
// underlying ADK constructors reject the configuration.
func New(model adkmodel.LLM, opts ...Option) (*Agent, error) {
	if model == nil {
		return nil, fmt.Errorf("agent: model is required")
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	inner, err := llmagent.New(llmagent.Config{
		Name:        o.name,
		Model:       model,
		Description: o.description,
		Instruction: o.instruction,
		Tools:       o.tools,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: build llmagent: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:           AppName,
		Agent:             inner,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: build runner: %w", err)
	}

	return &Agent{
		inner:     inner,
		runner:    r,
		streaming: o.streaming,
		userID:    o.userID,
		sessionID: o.sessionID,
	}, nil
}

// Run executes one turn of the agent against prompt and returns the event
// iterator straight from ADK's runner. Callers are expected to range over
// the returned iter.Seq2 and consume events as they arrive — partial text
// chunks, tool calls (none in Slice 1), and the final TurnComplete event.
func (a *Agent) Run(ctx context.Context, prompt string) iter.Seq2[*session.Event, error] {
	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	return a.runner.Run(ctx, a.userID, a.sessionID, msg, adkagent.RunConfig{
		StreamingMode: a.streaming,
	})
}
