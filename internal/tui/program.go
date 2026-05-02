package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

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

	m := NewModel(cfg, a)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		return ExitRunError, fmt.Errorf("tui: %w", err)
	}
	return ExitOK, nil
}
