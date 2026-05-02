// Package headless implements `cogo -p "prompt"` — a one-shot, no-TUI
// invocation that streams the assistant's response to stdout and exits.
//
// Designed for shell pipelines, CI, and quick tests; the same agent loop
// drives the interactive TUI in Slice 2.
package headless

import (
	"context"
	"fmt"
	"io"

	adkmodel "google.golang.org/adk/model"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
)

// Exit codes — kept distinct so CI can disambiguate failure modes.
const (
	ExitOK         = 0
	ExitAgentError = 1
	ExitConfigError = 2
)

// Run executes prompt against m and streams the assistant's text to
// stdout as partial events arrive. Tool-call summaries (none in Slice 1)
// would be written to stderr. Returns an exit code suitable for os.Exit.
//
// A trailing newline is always added to stdout when at least one chunk
// was written, so shell pipelines see a clean terminator.
func Run(ctx context.Context, m adkmodel.LLM, prompt string, stdout, stderr io.Writer) (int, error) {
	if prompt == "" {
		return ExitConfigError, fmt.Errorf("headless: prompt is required")
	}

	a, err := agent.New(m)
	if err != nil {
		return ExitAgentError, err
	}

	wroteAnything := false
	for event, err := range a.Run(ctx, prompt) {
		if err != nil {
			if wroteAnything {
				_, _ = fmt.Fprintln(stdout) // flush trailing newline before error
			}
			return ExitAgentError, fmt.Errorf("headless: agent run: %w", err)
		}
		if event.Content == nil {
			continue
		}
		// Partial events stream incremental text; we forward those directly.
		// The final TurnComplete event repeats the full text and is skipped
		// to avoid duplicate output.
		if !event.Partial {
			continue
		}
		for _, p := range event.Content.Parts {
			if p.Text == "" {
				continue
			}
			if _, err := io.WriteString(stdout, p.Text); err != nil {
				return ExitAgentError, fmt.Errorf("headless: write stdout: %w", err)
			}
			wroteAnything = true
		}
	}
	if wroteAnything {
		if _, err := fmt.Fprintln(stdout); err != nil {
			return ExitAgentError, fmt.Errorf("headless: write final newline: %w", err)
		}
	}
	return ExitOK, nil
}

// RunFromConfig is the entry point used by cmd/cogo: it builds a Provider
// from cfg, asks for the configured model ID, and dispatches to Run.
// Surfaces config-vs-runtime errors with distinct exit codes.
func RunFromConfig(ctx context.Context, cfg *config.Config, prompt string, stdout, stderr io.Writer) (int, error) {
	provider, err := models.Resolve(cfg)
	if err != nil {
		return ExitConfigError, err
	}
	m, err := provider.Model(ctx, cfg.Model.Name)
	if err != nil {
		return ExitConfigError, err
	}
	return Run(ctx, m, prompt, stdout, stderr)
}
