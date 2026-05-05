// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package permissions

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-steer/cogo/internal/config"
)

// ApprovalLog is one entry in the gate's per-session approval audit.
// It records every interactive permission decision the user made
// (excluding denials) so the TUI can later offer a "review approvals
// + recommend" workflow via the /permissions slash command.
type ApprovalLog struct {
	Tool     string
	Key      string
	Decision Decision
	At       time.Time
}

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
	// Tool-wide in-session allow set, keyed by tool name only.
	// Populated by DecisionAllowSessionTool when the user trusts an
	// entire tool for the rest of the session ("allow every read_file
	// call regardless of path"). Bash denylist still applies — that
	// pre-check runs before the gate ever sees the request.
	sessionAllowTools map[string]struct{}

	// Chronological log of every non-deny interactive approval. Surfaces
	// via Approvals() so /permissions can recommend allowlist entries
	// based on what the user actually approved this session.
	approvals []ApprovalLog
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
		mode:              opts.Mode,
		policy:            opts.Policy,
		scope:             opts.Scope,
		prompter:          opts.Prompter,
		sessionAllow:      make(map[string]struct{}),
		sessionAllowTools: make(map[string]struct{}),
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

// CheckGeneric gates an arbitrary tool call (used by MCP and skill
// toolsets, where we don't have a dedicated Check<Tool> method).
//
// toolName is the namespace under which policy lookups happen
// (typically "mcp" or "skill"); key is the human-readable detail
// shown in prompts (typically the tool's full namespaced name plus
// a brief argument summary).
func (g *Gate) CheckGeneric(ctx context.Context, toolName, key string) error {
	return g.gateRequest(ctx, PromptKindGeneric, toolName, key, toolName, key)
}

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
	// "Allow this tool · session" trusts the tool entirely for the
	// session, including out-of-scope paths. Short-circuit before the
	// scope check so the user doesn't get re-prompted.
	if g.sessionToolAllowed(toolName) {
		return nil
	}
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
	// "Allow this tool · session" trusts the tool entirely (mirrors
	// CheckFileRead). The user opted in explicitly for the rest of
	// the session.
	if g.sessionToolAllowed(toolName) {
		return nil
	}
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
	// Tool-wide session allow short-circuits before per-key session
	// allow so a user who picked "allow this tool · session" never
	// sees another modal for the same tool.
	if g.sessionToolAllowed(toolName) {
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
		g.recordApproval(req.ToolName, req.Detail, d)
		return nil
	case DecisionAllowSession:
		g.rememberSession(req.ToolName, req.Detail)
		g.recordApproval(req.ToolName, req.Detail, d)
		return nil
	case DecisionAllowSessionTool:
		// "Trust the whole tool for this session." We remember the
		// per-key entry too so any in-flight retry of the same call
		// doesn't re-prompt before the tool-wide entry takes effect.
		g.rememberSessionTool(req.ToolName)
		g.rememberSession(req.ToolName, req.Detail)
		g.recordApproval(req.ToolName, req.Detail, d)
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
		g.recordApproval(req.ToolName, req.Detail, d)
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

// sessionToolAllowed reports whether the user has trusted toolName
// entirely for this session via DecisionAllowSessionTool.
func (g *Gate) sessionToolAllowed(toolName string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.sessionAllowTools[toolName]
	return ok
}

func (g *Gate) rememberSessionTool(toolName string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sessionAllowTools[toolName] = struct{}{}
}

// recordApproval appends an interactive approval to the session's
// audit log. /permissions reads this back to recommend allowlist
// entries based on what the user actually approved.
func (g *Gate) recordApproval(toolName, key string, d Decision) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.approvals = append(g.approvals, ApprovalLog{
		Tool:     toolName,
		Key:      key,
		Decision: d,
		At:       time.Now(),
	})
}

// Approvals returns a defensive copy of the in-session approval log.
// Order is chronological. Safe for concurrent callers.
func (g *Gate) Approvals() []ApprovalLog {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]ApprovalLog, len(g.approvals))
	copy(out, g.approvals)
	return out
}
