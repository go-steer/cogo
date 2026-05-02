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
	"os"

	adkmodel "google.golang.org/adk/model"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
	"github.com/go-steer/cogo/internal/permissions"
	"github.com/go-steer/cogo/internal/tools"
)

// Exit codes — kept distinct so CI can disambiguate failure modes.
const (
	ExitOK         = 0
	ExitAgentError = 1
	ExitConfigError = 2
)

// Run executes prompt against m and streams the assistant's text to
// stdout as partial events arrive. Tool-call summaries are written to
// stderr as one line per call. Returns an exit code suitable for os.Exit.
//
// agentOpts lets the caller pass extra agent.Options (notably WithTools);
// when nil, the agent runs with no tools — useful for tests with FakeModel.
//
// A trailing newline is always added to stdout when at least one chunk
// was written, so shell pipelines see a clean terminator.
func Run(ctx context.Context, m adkmodel.LLM, prompt string, stdout, stderr io.Writer, agentOpts ...agent.Option) (int, error) {
	if prompt == "" {
		return ExitConfigError, fmt.Errorf("headless: prompt is required")
	}

	a, err := agent.New(m, agentOpts...)
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
		// Tool-call summaries → stderr (one line per call/result).
		// Partial assistant text → stdout (streamed incrementally).
		// Final TurnComplete event repeats the full text; skipped.
		for _, p := range event.Content.Parts {
			switch {
			case p.FunctionCall != nil:
				fmt.Fprintf(stderr, "→ %s\n", p.FunctionCall.Name)
			case p.FunctionResponse != nil:
				fmt.Fprintf(stderr, "← %s\n", p.FunctionResponse.Name)
			case p.Text != "" && event.Partial:
				if _, err := io.WriteString(stdout, p.Text); err != nil {
					return ExitAgentError, fmt.Errorf("headless: write stdout: %w", err)
				}
				wroteAnything = true
			}
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
// from cfg, asks for the configured model ID, assembles the built-in
// tools with a permission gate, and dispatches to Run.
//
// Headless invocations have no TTY for prompts, so the gate is built
// without a Prompter: anything that would prompt fails fast with a
// clear message asking the user to add an explicit allowlist entry.
func RunFromConfig(ctx context.Context, cfg *config.Config, prompt string, stdout, stderr io.Writer) (int, error) {
	provider, err := models.Resolve(cfg)
	if err != nil {
		return ExitConfigError, err
	}
	m, err := provider.Model(ctx, cfg.Model.Name)
	if err != nil {
		return ExitConfigError, err
	}
	cwd, _ := os.Getwd()
	userHome, _ := os.UserHomeDir()
	cogoHome := ""
	if userHome != "" {
		cogoHome = userHome + "/.cogo"
	}
	gate, err := permissions.FromConfig(cfg, cwd, cogoHome, nil /*no prompter*/)
	if err != nil {
		return ExitConfigError, err
	}
	registry, err := tools.Build(cfg, gate)
	if err != nil {
		return ExitConfigError, err
	}
	return Run(ctx, m, prompt, stdout, stderr, agent.WithTools(registry.Tools))
}
