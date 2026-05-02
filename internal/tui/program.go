package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/adk/tool"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/mcp"
	"github.com/go-steer/cogo/internal/memory"
	"github.com/go-steer/cogo/internal/models"
	"github.com/go-steer/cogo/internal/permissions"
	"github.com/go-steer/cogo/internal/skills"
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
// "Always allow" path-scope choices into config.json.
func Run(ctx context.Context, cfg *config.Config, agentsDir string) (int, error) {
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

	mcpServers, mcpToolsets, err := mcp.Build(ctx, agentsDir, send)
	if err != nil {
		earlyNotes = append(earlyNotes, "MCP load: "+err.Error())
	}
	skillsLoaded, err := skills.Load(ctx, agentsDir)
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

	// Note: deliberately NOT enabling tea.WithMouseCellMotion — capturing
	// mouse events globally breaks terminal-native text selection (copy /
	// paste) and makes mouse interaction in the input area feel wrong.
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)
	prompter.(*tuiPrompter).send = p

	if _, err := p.Run(); err != nil {
		return ExitRunError, fmt.Errorf("tui: %w", err)
	}
	return ExitOK, nil
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
