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
	"path/filepath"

	adkmodel "google.golang.org/adk/model"

	adktool "google.golang.org/adk/tool"

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
// agentOpts lets the caller pass extra agent.Options (notably WithTools
// and WithSystemInstructionPrefix). When nil, the agent runs with no
// tools and the default instruction — useful for tests with FakeModel.
//
// tracker (optional) records per-turn usage; when supplied, RunFromConfig
// writes an exit summary using its totals. Pass nil to skip accounting.
//
// A trailing newline is always added to stdout when at least one chunk
// was written, so shell pipelines see a clean terminator.
func Run(ctx context.Context, m adkmodel.LLM, prompt string, stdout, stderr io.Writer, tracker *usage.Tracker, pricing usage.Pricing, agentOpts ...agent.Option) (int, error) {
	if prompt == "" {
		return ExitConfigError, fmt.Errorf("headless: prompt is required")
	}

	a, err := agent.New(m, agentOpts...)
	if err != nil {
		return ExitAgentError, err
	}

	wroteAnything := false
	var lastUsageInput, lastUsageOutput int
	for event, err := range a.Run(ctx, prompt) {
		if err != nil {
			if wroteAnything {
				_, _ = fmt.Fprintln(stdout) // flush trailing newline before error
			}
			return ExitAgentError, fmt.Errorf("headless: agent run: %w", err)
		}
		// Capture usage metadata when present. Final events typically
		// carry the totals for the turn; partials may also have it.
		if event.UsageMetadata != nil {
			lastUsageInput = int(event.UsageMetadata.PromptTokenCount)
			lastUsageOutput = int(event.UsageMetadata.CandidatesTokenCount)
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
	if tracker != nil && (lastUsageInput > 0 || lastUsageOutput > 0) {
		tracker.Append(m.Name(), lastUsageInput, lastUsageOutput, pricing)
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
// tools with a permission gate, loads project + user memory, runs the
// turn, and writes a one-line cost/usage summary to stderr on success.
//
// Headless invocations have no TTY for prompts, so the gate is built
// without a Prompter: anything that would prompt fails fast with a
// clear message asking the user to add an explicit allowlist entry.
//
// agentsDir is the resolved .agents/ directory if discovered (else "");
// passing it lets the memory loader anchor the project search at the
// right root.
func RunFromConfig(ctx context.Context, cfg *config.Config, agentsDir, prompt string, stdout, stderr io.Writer) (int, error) {
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
		cogoHome = filepath.Join(userHome, ".cogo")
	}
	gate, err := permissions.FromConfig(cfg, cwd, cogoHome, nil /*no prompter*/)
	if err != nil {
		return ExitConfigError, err
	}
	registry, err := tools.Build(cfg, gate)
	if err != nil {
		return ExitConfigError, err
	}

	// Project memory anchors at agentsDir's parent (the project root)
	// when we have one; otherwise the cwd is the best guess.
	projectRoot := cwd
	if agentsDir != "" {
		projectRoot = filepath.Dir(agentsDir)
	}
	loaded, err := memory.Load(projectRoot, cogoHome)
	if err != nil {
		// Memory load failures are surfaced as warnings, not fatal:
		// the agent should still be usable without memory.
		fmt.Fprintf(stderr, "cogo: memory load: %v\n", err)
	}

	tracker := usage.NewTracker()
	pricing := usage.PriceFor(cfg.Model.Name, cfg)

	// MCP servers + skills: notes (server connect failures, elicitation
	// requests) flow to stderr in headless mode.
	send := func(s string) { fmt.Fprintln(stderr, "cogo: "+s) }
	_, mcpToolsets, mcpErr := mcp.Build(ctx, agentsDir, send)
	if mcpErr != nil {
		fmt.Fprintf(stderr, "cogo: mcp: %v\n", mcpErr)
	}
	skillsLoaded, skillsErr := skills.Load(ctx, agentsDir)
	if skillsErr != nil {
		fmt.Fprintf(stderr, "cogo: skills: %v\n", skillsErr)
	}
	allToolsets := append([]adktool.Toolset{}, mcpToolsets...)
	if !skillsLoaded.Empty() {
		allToolsets = append(allToolsets, skillsLoaded.Toolset)
	}

	opts := []agent.Option{
		agent.WithTools(registry.Tools),
		agent.WithToolsets(allToolsets),
		agent.WithSystemInstructionPrefix(loaded.Instruction),
	}
	code, err := Run(ctx, m, prompt, stdout, stderr, tracker, pricing, opts...)
	if err == nil && code == ExitOK {
		writeExitSummary(stderr, tracker, m.Name())
	}
	return code, err
}

// writeExitSummary emits a one-line usage tally suitable for shell
// pipelines / CI logs. No-op when no turns were recorded (e.g. the
// model returned no usage metadata).
func writeExitSummary(w io.Writer, t *usage.Tracker, modelID string) {
	tot := t.Totals()
	if tot.Turns == 0 {
		return
	}
	fmt.Fprintf(w, "cogo: %d turn(s) · ↑%d ↓%d tokens · $%.4f (%s)\n",
		tot.Turns, tot.InputTokens, tot.OutputTokens, tot.CostUSD, modelID)
}
