package mcp

import (
	"context"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// runnable is the unexported interface ADK's runner expects from
// tools that can actually be called. mcptoolset returns objects that
// satisfy it; we re-declare it locally so we can both type-assert and
// implement against it without importing an unexported symbol.
type runnable interface {
	Declaration() *genai.FunctionDeclaration
	Run(ctx tool.Context, args any) (result map[string]any, err error)
}

// namespacedToolset wraps an upstream Toolset and returns each Tool
// with its name prefixed by `<prefix>_`. This both:
//   - prevents collisions with Cogo's built-in tool names
//     (e.g. an MCP filesystem server's `read_file` would otherwise
//     duplicate the built-in `read_file`)
//   - keeps function names within Gemini's `[A-Za-z0-9_]{1,64}`
//     constraint (so `.` as a separator is not an option)
type namespacedToolset struct {
	inner  tool.Toolset
	prefix string
}

// withNamespace prefixes every tool name in inner with prefix + "_".
// Returns inner unchanged if prefix is empty.
func withNamespace(inner tool.Toolset, prefix string) tool.Toolset {
	if inner == nil || prefix == "" {
		return inner
	}
	return &namespacedToolset{inner: inner, prefix: sanitizePrefix(prefix)}
}

func (n *namespacedToolset) Name() string {
	if base := n.inner.Name(); base != "" {
		return n.prefix + "_" + base
	}
	return n.prefix
}

func (n *namespacedToolset) Tools(ctx agent.ReadonlyContext) ([]tool.Tool, error) {
	upstream, err := n.inner.Tools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tool.Tool, 0, len(upstream))
	for _, t := range upstream {
		out = append(out, renamedTool{inner: t, prefix: n.prefix})
	}
	return out, nil
}

// renamedTool exposes the underlying tool with a prefixed Name so the
// agent sees `<prefix>_<original>` everywhere. Description and
// long-running flag pass through; Run delegates verbatim.
type renamedTool struct {
	inner  tool.Tool
	prefix string
}

func (r renamedTool) Name() string {
	return r.prefix + "_" + r.inner.Name()
}

func (r renamedTool) Description() string { return r.inner.Description() }

func (r renamedTool) IsLongRunning() bool { return r.inner.IsLongRunning() }

// Declaration delegates to the underlying tool's runnable declaration
// (when it has one) but rewrites the function name to the prefixed
// form. Returns a fresh struct so we don't mutate the upstream copy.
func (r renamedTool) Declaration() *genai.FunctionDeclaration {
	rn, ok := r.inner.(runnable)
	if !ok {
		return nil
	}
	d := rn.Declaration()
	if d == nil {
		return nil
	}
	clone := *d
	clone.Name = r.Name()
	return &clone
}

// Run delegates to the underlying tool unchanged. The renamed name
// only matters when the agent advertises the tool; once the call
// arrives here, the underlying tool just runs.
func (r renamedTool) Run(ctx tool.Context, args any) (map[string]any, error) {
	rn, ok := r.inner.(runnable)
	if !ok {
		// Should never happen for tools we'd realistically wrap, but
		// give a deterministic error rather than panic on assertion.
		return nil, errNotRunnable
	}
	return rn.Run(ctx, args)
}

// sanitizePrefix normalizes a server name into a Gemini-friendly
// identifier prefix: keeps [A-Za-z0-9_], replaces everything else
// with `_`. Caller passes mcp.json server keys, which users may
// have written with hyphens or other separators.
func sanitizePrefix(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// errNotRunnable is returned when a wrapped tool unexpectedly
// doesn't satisfy the runnable interface. Surfaced by the agent
// loop as a normal tool error.
var errNotRunnable = simpleErr("mcp: wrapped tool does not implement runnable interface")

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

// Compile-time assertion that ctx in Run matches what tools expect.
var _ context.Context = (tool.Context)(nil)
