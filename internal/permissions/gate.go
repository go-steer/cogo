package permissions

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-steer/cogo/internal/config"
)

// Mode mirrors the permission modes documented in REQUIREMENTS §3.10.
type Mode string

const (
	ModeAsk   Mode = "ask"
	ModeAllow Mode = "allow"
	ModeYolo  Mode = "yolo"
)

// Gate is the central permission chokepoint consulted before each tool
// call. It holds the configured policy, the path scope, the bash
// denylist (built-in), and an optional Prompter for interactive use.
//
// Gate is safe for concurrent use; tool handlers run in the agent's
// event-iteration goroutine which is single-threaded today, but the
// prompter call may yield while waiting for the user.
type Gate struct {
	mu sync.Mutex

	mode     Mode
	policy   *Policy
	scope    *PathScope
	prompter Prompter

	// In-session allow set keyed by tool|key. Populated by
	// DecisionAllowSession choices so we don't re-prompt the same call
	// repeatedly within one session.
	sessionAllow map[string]struct{}
}

// Options configures a Gate at construction time. All fields are
// optional; sensible defaults apply when omitted.
type Options struct {
	Mode     Mode
	Policy   *Policy
	Scope    *PathScope
	Prompter Prompter // nil = no interactive path; ask-mode unresolved → deny
}

// New builds a Gate from the supplied options. The Mode defaults to
// "ask"; missing Policy/Scope default to permissive empties.
func New(opts Options) *Gate {
	if opts.Mode == "" {
		opts.Mode = ModeAsk
	}
	if opts.Policy == nil {
		opts.Policy, _ = NewPolicy(nil, nil)
	}
	if opts.Scope == nil {
		opts.Scope, _ = NewPathScope("", "", nil)
	}
	return &Gate{
		mode:         opts.Mode,
		policy:       opts.Policy,
		scope:        opts.Scope,
		prompter:     opts.Prompter,
		sessionAllow: make(map[string]struct{}),
	}
}

// FromConfig builds a Gate from a Cogo config plus the resolved
// project root and user-global root. The Prompter is wired separately
// since it depends on whether we're running TUI or headless.
func FromConfig(cfg *config.Config, projectRoot, userRoot string, prompter Prompter) (*Gate, error) {
	policy, err := NewPolicy(cfg.Permissions.Allow, cfg.Permissions.Deny)
	if err != nil {
		return nil, fmt.Errorf("permissions policy: %w", err)
	}
	scope, err := NewPathScope(projectRoot, userRoot, cfg.PathScope.Allow)
	if err != nil {
		return nil, err
	}
	mode := Mode(cfg.Permissions.Mode)
	if mode == "" {
		mode = ModeAsk
	}
	return New(Options{Mode: mode, Policy: policy, Scope: scope, Prompter: prompter}), nil
}

// Mode reports the active permission mode.
func (g *Gate) Mode() Mode { return g.mode }

// Scope exposes the path scope. Callers that mutate the scope should
// also persist the change via the config layer.
func (g *Gate) Scope() *PathScope { return g.scope }

// CheckBash gates a bash invocation. The denylist is checked first and
// is non-overridable. After that, policy + mode determine whether the
// call needs a prompt.
func (g *Gate) CheckBash(ctx context.Context, command string) error {
	command = strings.TrimSpace(command)
	if denied, reason := IsBashDenied(command); denied {
		return fmt.Errorf("bash refused: %s", reason)
	}
	return g.gateRequest(ctx, PromptKindBash, "bash", command, "bash", command)
}

// CheckFileRead gates a read-only file operation. Read access only
// fails if the path is out of scope (and the user can't or won't
// extend scope).
func (g *Gate) CheckFileRead(ctx context.Context, toolName, path string) error {
	in, err := g.scope.Contains(path)
	if err != nil {
		return err
	}
	if in {
		return nil
	}
	return g.promptForPath(ctx, toolName, path, "read")
}

// CheckFileWrite gates a mutating file operation. Out-of-scope paths
// are escalated via prompt; in-scope paths still go through mode-aware
// approval (ask mode prompts; allow/yolo proceed unless deny rule hits).
func (g *Gate) CheckFileWrite(ctx context.Context, toolName, path string) error {
	in, err := g.scope.Contains(path)
	if err != nil {
		return err
	}
	if !in {
		return g.promptForPath(ctx, toolName, path, "write")
	}
	return g.gateRequest(ctx, PromptKindFileWrite, toolName, path, toolName, path)
}

// gateRequest applies the policy + mode chain to a request that has
// already cleared any tool-specific pre-checks (denylist, scope).
func (g *Gate) gateRequest(ctx context.Context, kind PromptKind, toolName, key, persistTool, persistKey string) error {
	switch g.policy.Match(toolName, key) {
	case OutcomeDeny:
		return fmt.Errorf("%s denied by config policy: %q", toolName, key)
	case OutcomeAllow:
		return nil
	}
	if g.sessionAllowed(toolName, key) {
		return nil
	}
	switch g.mode {
	case ModeYolo:
		return nil
	case ModeAllow:
		// In allow mode we only proceed when policy explicitly allowed
		// (handled above) — otherwise deny without prompting.
		return fmt.Errorf("%s requires an allowlist entry in 'allow' mode: %q", toolName, key)
	case ModeAsk:
		return g.prompt(ctx, PromptRequest{
			Kind:        kind,
			ToolName:    toolName,
			Detail:      key,
			PersistTool: persistTool,
			PersistKey:  persistKey,
		})
	}
	return fmt.Errorf("%s denied: unknown permission mode %q", toolName, g.mode)
}

// promptForPath escalates an out-of-scope file access. The persistKey
// is the path itself; if the user picks "always allow", a directory
// pattern (or exact-path for files) is appended to path_scope.allow.
func (g *Gate) promptForPath(ctx context.Context, toolName, path, op string) error {
	if g.mode == ModeYolo {
		return nil
	}
	if g.mode == ModeAllow {
		return fmt.Errorf("%s denied: path %q is outside scope and 'allow' mode does not prompt", toolName, path)
	}
	return g.prompt(ctx, PromptRequest{
		Kind:        PromptKindPathScope,
		ToolName:    toolName,
		Detail:      fmt.Sprintf("%s %s (out of scope)", op, path),
		PersistTool: "path_scope",
		PersistKey:  path,
	})
}

// prompt invokes the configured Prompter and acts on the user's
// decision. With no Prompter the call fails fast with ErrNoPrompter so
// callers (notably headless mode) can surface a clear error.
func (g *Gate) prompt(ctx context.Context, req PromptRequest) error {
	if g.prompter == nil {
		return fmt.Errorf("%w (tool=%s detail=%q); set permissions.mode=\"allow\" with explicit allowlist entries for headless use", ErrNoPrompter, req.ToolName, req.Detail)
	}
	d, err := g.prompter.AskApproval(ctx, req)
	if err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	switch d {
	case DecisionAllowOnce:
		return nil
	case DecisionAllowSession:
		g.rememberSession(req.ToolName, req.Detail)
		return nil
	case DecisionAllowAlways:
		// Caller (e.g. TUI host) is responsible for persisting the
		// pattern via the config layer; the gate also remembers it
		// for the rest of this session so the next call doesn't
		// re-prompt before persistence completes.
		g.rememberSession(req.ToolName, req.Detail)
		if req.Kind == PromptKindPathScope {
			g.scope.AddAlwaysAllow(req.PersistKey)
		}
		return nil
	default:
		return fmt.Errorf("%s denied by user: %s", req.ToolName, req.Detail)
	}
}

func (g *Gate) sessionAllowed(toolName, key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.sessionAllow[toolName+"|"+key]
	return ok
}

func (g *Gate) rememberSession(toolName, key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessionAllow[toolName+"|"+key] = struct{}{}
}
