package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
	"github.com/go-steer/cogo/internal/permissions"
	"github.com/go-steer/cogo/internal/tools"
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
	a, err := agent.New(llm, agent.WithTools(registry.Tools))
	if err != nil {
		return ExitConfigError, err
	}
	m.agent = a
	m.scope = gate.Scope()

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
