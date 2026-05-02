package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"

	"github.com/go-steer/cogo/internal/permissions"
	cogotools "github.com/go-steer/cogo/internal/tools"
)

// toolsGateToolset is a tiny indirection so the import alias stays
// readable inside startOne — internal/tools and the local "tools"
// alias of mcptoolset don't collide visually.
var toolsGateToolset = cogotools.GateToolset

// Status values surfaced via /mcp.
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// Server is one configured MCP server's runtime state.
type Server struct {
	Name    string
	Status  string
	Tools   []string // tool names exposed; populated lazily by Toolset
	Err     error    // non-nil when Status == StatusError
	toolset tool.Toolset
	cmd     *exec.Cmd // stdio child; nil for http transports
}

// Toolset returns the MCP toolset, or nil for failed servers. Used by
// program.go to feed agent.WithToolsets.
func (s *Server) Toolset() tool.Toolset { return s.toolset }

// Close terminates any child process this server owns. Called from
// /reload before swapping in a fresh generation of servers, so stdio
// MCP children don't pile up across reloads.
//
// For HTTP transports there's no process to kill — Close is a no-op.
//
// Termination strategy: SIGTERM, give the process up to 3 seconds to
// exit gracefully, then SIGKILL. Wait is called either way to reap
// the zombie. Errors (process already exited, etc.) are swallowed —
// the only thing the caller can do with them is log, and we have no
// logger here.
func (s *Server) Close() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = s.cmd.Process.Wait()
		close(done)
	}()
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}

// Build reads .agents/mcp.json and starts every declared server in
// parallel. The send callback is plumbed into each server's elicitation
// handler (when no interactive elicitor is provided) so Cogo can
// surface elicitation requests in the right place (TUI system message
// vs headless stderr).
//
// gate (optional) gates each MCP tool call through Cogo's permission
// system so MCP tools are subject to the same ask/allow/yolo rules
// as built-in tools. Pass nil to skip gating.
//
// elicitor (optional) is the interactive bridge for elicitation
// requests. The TUI passes a function that opens a modal and blocks
// on the user's answer; headless callers leave it nil and fall back
// to DeclineHandler.
//
// Servers that fail to start come back with Status==StatusError so
// they're visible in /mcp without breaking the rest of the agent.
func Build(ctx context.Context, agentsDir string, send func(string), gate *permissions.Gate, elicitor ElicitorFn) ([]*Server, []tool.Toolset, error) {
	cfg, err := Load(agentsDir)
	if err != nil {
		return nil, nil, err
	}
	if len(cfg.Servers) == 0 {
		return nil, nil, nil
	}

	out := make([]*Server, 0, len(cfg.Servers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, spec := range cfg.Servers {
		wg.Add(1)
		go func(name string, spec ServerSpec) {
			defer wg.Done()
			srv := startOne(ctx, name, spec, send, gate, elicitor)
			mu.Lock()
			out = append(out, srv)
			mu.Unlock()
		}(name, spec)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	toolsets := make([]tool.Toolset, 0, len(out))
	for _, s := range out {
		if s.toolset != nil {
			toolsets = append(toolsets, s.toolset)
		}
	}
	return out, toolsets, nil
}

// startOne instantiates one server. Errors are stored on the Server
// rather than returned so a single broken server doesn't prevent the
// rest of the registry from coming up.
func startOne(ctx context.Context, name string, spec ServerSpec, send func(string), gate *permissions.Gate, elicitor ElicitorFn) *Server {
	srv := &Server{Name: name}

	transport, cmd, err := transportFor(spec)
	if err != nil {
		srv.Status = StatusError
		srv.Err = err
		return srv
	}
	srv.cmd = cmd

	client := mcpsdk.NewClient(
		&mcpsdk.Implementation{Name: "cogo", Version: "0.1.0"},
		&mcpsdk.ClientOptions{ElicitationHandler: handlerFor(name, send, elicitor)},
	)
	ts, err := mcptoolset.New(mcptoolset.Config{
		Client:    client,
		Transport: transport,
	})
	if err != nil {
		srv.Status = StatusError
		srv.Err = fmt.Errorf("toolset: %w", err)
		return srv
	}
	// Wrap with our own namespace so an MCP server's `read_file` (for
	// example) doesn't collide with Cogo's built-in `read_file`.
	// Underscore separator because Gemini's function-name regex is
	// `[A-Za-z0-9_]{1,64}` — a `.` would be rejected.
	wrapped := withNamespace(ts, name)
	// Then wrap with the permission gate so MCP tool calls go through
	// the same ask/allow/yolo flow as built-in tools. Allowlist
	// patterns use the "mcp" namespace, e.g. "mcp:filesystem_read_file".
	if gate != nil {
		wrapped = toolsGateToolset(wrapped, gate, "mcp")
	}
	srv.toolset = wrapped
	srv.Status = StatusOK
	// Enumerate tool names so /mcp can list them. We pass a minimal
	// listCtx since mcptoolset's Tools() implementation uses ctx
	// solely as a context.Context for the underlying network call.
	if tools, err := wrapped.Tools(asReadonly(ctx)); err == nil {
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, t.Name())
		}
		sort.Strings(names)
		srv.Tools = names
	}
	return srv
}

// transportFor builds the appropriate mcp.Transport for the spec.
// For stdio it also returns the *exec.Cmd so the Server can hold a
// reference for shutdown; for http the cmd is nil.
func transportFor(spec ServerSpec) (mcpsdk.Transport, *exec.Cmd, error) {
	switch spec.Transport {
	case "stdio":
		// Spec is sourced from the user's own .agents/mcp.json; spawning
		// the configured command is the contract.
		cmd := exec.Command(spec.Command, spec.Args...) // #nosec G204
		// Apply env interpolation; only set non-empty values so we
		// don't accidentally clobber inherited env with empty strings.
		env := InterpolateMap(spec.Env)
		if len(env) > 0 {
			// Inherit parent env, then layer ours on top.
			cmd.Env = append(cmd.Env, append([]string{}, parentEnv()...)...)
			for k, v := range env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}
		return &mcpsdk.CommandTransport{Command: cmd}, cmd, nil
	case "http":
		headers := InterpolateMap(spec.Headers)
		client := &http.Client{}
		rt := http.DefaultTransport
		if len(headers) > 0 {
			rt = &headerTransport{base: rt, headers: headers}
		}
		client.Transport = rt
		return &mcpsdk.StreamableClientTransport{
			Endpoint:   spec.URL,
			HTTPClient: client,
		}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown transport %q", spec.Transport)
	}
}

// headerTransport injects custom headers into every outgoing request.
// Used for MCP HTTP servers that authenticate via headers.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for k, v := range t.headers {
		if v != "" {
			clone.Header.Set(k, v)
		}
	}
	return t.base.RoundTrip(clone)
}

// parentEnv returns os.Environ() — split out for testability without
// pulling os into call sites that don't otherwise need it.
func parentEnv() []string {
	return osEnviron()
}
