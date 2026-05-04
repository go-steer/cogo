// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/adk/tool"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/mcp"
	"github.com/go-steer/cogo/internal/memory"
	"github.com/go-steer/cogo/internal/models"
	"github.com/go-steer/cogo/internal/permissions"
	"github.com/go-steer/cogo/internal/session"
	"github.com/go-steer/cogo/internal/skills"
	"github.com/go-steer/cogo/internal/telemetry"
	"github.com/go-steer/cogo/internal/tools"
	"github.com/go-steer/cogo/internal/usage"
)

// Exit codes used by the headless package — re-exported here so cmd/cogo
// can use one set of constants regardless of which mode it dispatched to.
const (
	ExitOK          = 0
	ExitRunError    = 1
	ExitConfigError = 2
)

// Run launches the TUI bound to cfg. It blocks until the user quits.
//
// Resolution failures (no auth, bad config) return ExitConfigError so
// the caller can distinguish them from runtime errors. Runtime errors
// from Bubble Tea itself return ExitRunError.
//
// agentsDir is the resolved .agents/ directory if one was found, or
// empty when running with built-in defaults; we use it to persist
// "Always allow" path-scope choices into config.json and to write
// session transcripts under sessions/.
func Run(ctx context.Context, cfg *config.Config, agentsDir string) (int, error) {
	startedAt := time.Now()

	// OpenTelemetry — off by default; honors cfg.OTEL.Exporter.
	otelShutdown, err := telemetry.Setup(ctx, cfg.OTEL.Exporter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cogo: telemetry setup: %v\n", err)
	}
	defer func() { _ = otelShutdown(context.Background()) }()

	provider, err := models.Resolve(cfg)
	if err != nil {
		return ExitConfigError, err
	}
	llm, err := provider.Model(ctx, cfg.Model.Name)
	if err != nil {
		return ExitConfigError, err
	}

	// Detect terminal background BEFORE tea.NewProgram takes over stdin.
	// Glamour's WithAutoStyle sends an OSC-11 query whose response
	// would otherwise race into the textarea as input. Resolving the
	// style name once up front and threading it through NewModel keeps
	// every Glamour rebuild (resize, etc.) silent.
	mdStyle := "dark"
	if !lipgloss.HasDarkBackground() {
		mdStyle = "light"
	}

	cwd, _ := os.Getwd()
	userHome, _ := os.UserHomeDir()
	cogoHome := ""
	if userHome != "" {
		cogoHome = filepath.Join(userHome, ".cogo")
	}

	// We don't have the model yet — first construct it, then attach
	// the prompter that send-cmsgs into it. Tools must be built last
	// because their handlers close over the gate.
	m := NewModel(cfg, nil, mdStyle)
	prompter := NewPrompter(nil) // wired after p is built
	elicitor := newTUIElicitor() // wired after p is built

	gate, err := permissions.FromConfig(cfg, cwd, cogoHome, prompter)
	if err != nil {
		return ExitConfigError, err
	}
	registry, err := tools.Build(cfg, gate)
	if err != nil {
		return ExitConfigError, err
	}

	// Load project + user memory; failure is non-fatal but surfaced.
	projectRoot := cwd
	if agentsDir != "" {
		projectRoot = filepath.Dir(agentsDir)
	}
	loaded, err := memory.Load(projectRoot, cogoHome)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cogo: memory load: %v\n", err)
	}

	// MCP servers + skills add to the toolsets the agent can use.
	// Any failure here surfaces as a system message later (via
	// /mcp / /skills) rather than blocking startup. We need a place
	// to write those messages; before the program exists we collect
	// them and replay into the model right after construction.
	var earlyNotes []string
	send := func(s string) { earlyNotes = append(earlyNotes, s) }

	mcpServers, mcpToolsets, err := mcp.Build(ctx, agentsDir, send, gate, elicitor.Elicit)
	if err != nil {
		earlyNotes = append(earlyNotes, "MCP load: "+err.Error())
	}
	skillsLoaded, err := skills.Load(ctx, agentsDir, gate)
	if err != nil {
		earlyNotes = append(earlyNotes, "Skills load: "+err.Error())
	}

	allToolsets := append([]tool.Toolset{}, mcpToolsets...)
	if !skillsLoaded.Empty() {
		allToolsets = append(allToolsets, skillsLoaded.Toolset)
	}

	a, err := agent.New(llm,
		agent.WithTools(registry.Tools),
		agent.WithToolsets(allToolsets),
		agent.WithSystemInstructionPrefix(loaded.Instruction),
	)
	if err != nil {
		return ExitConfigError, err
	}
	m.agent = a
	m.scope = gate.Scope()
	m.memory = loaded
	m.usage = usage.NewTracker()
	m.mcpServers = mcpServers
	m.skills = skillsLoaded
	for _, note := range earlyNotes {
		m.history.Append(Message{Role: RoleSystem, Text: note})
	}

	// rebuildAgent lets /model swap the model mid-session without the
	// TUI having to know about the provider, gate, or tools layout.
	m.rebuildAgent = func(modelID string) (*agent.Agent, error) {
		newLLM, err := provider.Model(ctx, modelID)
		if err != nil {
			return nil, err
		}
		return agent.New(newLLM,
			agent.WithTools(registry.Tools),
			agent.WithToolsets(allToolsets),
			agent.WithSystemInstructionPrefix(loaded.Instruction),
		)
	}
	if agentsDir != "" {
		m.persistModelChoice = func(modelID string) error {
			c, err := config.Load(agentsDir)
			if err != nil {
				return err
			}
			c.Model.Name = modelID
			return config.Save(filepath.Join(agentsDir, config.ConfigFileName), c)
		}
	}

	// /reload pulls everything fresh from disk: memory + MCP + skills.
	// We only offer it when there's a project root to read from.
	if agentsDir != "" {
		m.reloadFromDisk = func() (reloadResult, error) {
			newMemory, err := memory.Load(projectRoot, cogoHome)
			if err != nil {
				return reloadResult{}, fmt.Errorf("memory: %w", err)
			}
			// Tear down the previous generation of MCP servers BEFORE
			// starting the new ones so stdio child processes get
			// reaped instead of leaking. Closing the model's current
			// servers is safe because /reload only fires from the
			// program goroutine that also owns the model.
			for _, old := range m.mcpServers {
				old.Close()
			}
			newMCPServers, newMCPToolsets, err := mcp.Build(ctx, agentsDir, send, gate, elicitor.Elicit)
			if err != nil {
				return reloadResult{}, fmt.Errorf("mcp: %w", err)
			}
			newSkills, err := skills.Load(ctx, agentsDir, gate)
			if err != nil {
				return reloadResult{}, fmt.Errorf("skills: %w", err)
			}
			toolsets := append([]tool.Toolset{}, newMCPToolsets...)
			if !newSkills.Empty() {
				toolsets = append(toolsets, newSkills.Toolset)
			}
			newLLM, err := provider.Model(ctx, m.cfg.Model.Name)
			if err != nil {
				return reloadResult{}, fmt.Errorf("model: %w", err)
			}
			newAgent, err := agent.New(newLLM,
				agent.WithTools(registry.Tools),
				agent.WithToolsets(toolsets),
				agent.WithSystemInstructionPrefix(newMemory.Instruction),
			)
			if err != nil {
				return reloadResult{}, fmt.Errorf("agent: %w", err)
			}
			return reloadResult{
				Agent:      newAgent,
				Memory:     newMemory,
				MCPServers: newMCPServers,
				Skills:     newSkills,
			}, nil
		}
	}

	// Hook for "Always allow" persistence. For Slice 3 we only persist
	// path-scope additions to .agents/config.json; bash and other
	// allowlist additions arrive in Slice 4 alongside the full /model
	// + /init slash-command surface.
	m.AlwaysAllow = func(req permissions.PromptRequest) error {
		if req.PersistTool != "path_scope" || agentsDir == "" {
			// Nothing to persist when we don't have a project root.
			return nil
		}
		return appendPathScope(agentsDir, req.PersistKey)
	}

	// Mouse capture wires the wheel to viewport scrolling. Capturing
	// mouse events also takes plain click-drag away from the terminal's
	// native text selection, so users hold Shift to select. Disable via
	// `ui.mouse: false` in config or /mouse off at runtime.
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	m.mouseEnabled = cfg.UI.MouseEnabled()
	if m.mouseEnabled {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	m.SetProgram(p)
	prompter.(*tuiPrompter).send = p
	elicitor.attach(p)

	finalModel, err := p.Run()
	if err != nil {
		return ExitRunError, fmt.Errorf("tui: %w", err)
	}

	if fm, ok := finalModel.(*Model); ok {
		// Reap stdio MCP children before we exit. They'd be reaped by
		// init eventually anyway, but doing it here keeps the
		// process-leak window small and makes `ps` cleaner.
		for _, srv := range fm.mcpServers {
			srv.Close()
		}
		// Persist transcript on exit when we have a project root.
		// Failures are non-fatal; we report to stderr so they're
		// visible after the alt-screen is torn down.
		if agentsDir != "" {
			path, err := saveTranscript(agentsDir, startedAt, fm)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cogo: transcript save: %v\n", err)
			} else if path != "" {
				fmt.Fprintf(os.Stderr, "cogo: transcript saved to %s\n", path)
			}
		}
	}
	return ExitOK, nil
}

// saveTranscript serializes the TUI's chat history + usage totals to
// .agents/sessions/<timestamp>.json.
func saveTranscript(agentsDir string, started time.Time, m *Model) (string, error) {
	msgs := m.history.Snapshot()
	out := make([]session.Message, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, session.Message{Role: roleString(msg.Role), Text: msg.Text})
	}
	tot := session.Usage{}
	if m.usage != nil {
		t := m.usage.Totals()
		tot = session.Usage{Turns: t.Turns, InputTokens: t.InputTokens, OutputTokens: t.OutputTokens, CostUSD: t.CostUSD}
	}
	return session.Save(agentsDir, session.Transcript{
		StartedAt: started,
		Model:     m.cfg.Model.Name,
		Messages:  out,
		Usage:     tot,
	})
}

// roleString maps the TUI's Role enum to the human-readable strings
// the transcript schema uses.
func roleString(r Role) string {
	switch r {
	case RoleUser:
		return "user"
	case RoleAssistant:
		return "assistant"
	case RoleSystem:
		return "system"
	case RoleError:
		return "error"
	}
	return "unknown"
}

// appendPathScope adds pattern to .agents/config.json's
// path_scope.allow list and rewrites the file atomically. If the file
// doesn't exist yet it is created with defaults so the addition has
// somewhere to live.
func appendPathScope(agentsDir, pattern string) error {
	cfg, err := config.Load(agentsDir)
	if err != nil {
		return err
	}
	for _, existing := range cfg.PathScope.Allow {
		if existing == pattern {
			return nil
		}
	}
	cfg.PathScope.Allow = append(cfg.PathScope.Allow, pattern)
	return config.Save(filepath.Join(agentsDir, config.ConfigFileName), cfg)
}
