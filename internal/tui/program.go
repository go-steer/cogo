package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
)

// Exit codes used by the headless package — re-exported here so cmd/cogo
// can use one set of constants regardless of which mode it dispatched to.
const (
	ExitOK         = 0
	ExitRunError   = 1
	ExitConfigError = 2
)

// Run launches the TUI bound to cfg. It blocks until the user quits.
//
// Resolution failures (no auth, bad config) return ExitConfigError so
// the caller can distinguish them from runtime errors. Runtime errors
// from Bubble Tea itself return ExitRunError.
func Run(ctx context.Context, cfg *config.Config) (int, error) {
	provider, err := models.Resolve(cfg)
	if err != nil {
		return ExitConfigError, err
	}
	llm, err := provider.Model(ctx, cfg.Model.Name)
	if err != nil {
		return ExitConfigError, err
	}
	a, err := agent.New(llm)
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

	m := NewModel(cfg, a, mdStyle)
	// Note: deliberately NOT enabling tea.WithMouseCellMotion — capturing
	// mouse events globally breaks terminal-native text selection (copy /
	// paste) and makes mouse interaction in the input area feel wrong.
	// Scrolling is keyboard-driven (PgUp/PgDn, plus Up/Down when the
	// input is empty). Mouse-wheel scroll can come back later behind a
	// runtime toggle.
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		return ExitRunError, fmt.Errorf("tui: %w", err)
	}
	return ExitOK, nil
}
