// Package mcp wires user-configured Model Context Protocol servers
// into Cogo's agent loop.
//
// At startup the TUI / headless host calls Build, which reads
// .agents/mcp.json, spawns each declared server (stdio child or
// Streamable HTTP client), wraps the resulting MCP toolsets via ADK's
// google.golang.org/adk/tool/mcptoolset, and returns:
//
//   - the toolsets, so they can be passed to agent.New(WithToolsets…)
//   - per-server records the /mcp slash command renders.
//
// Failures are non-fatal: a server whose process won't start surfaces
// in /mcp with its error; the agent continues with whichever servers
// did connect.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// MCPFileName is the project-local MCP config file inside .agents/.
const MCPFileName = "mcp.json"

// Servers is the on-disk schema for .agents/mcp.json.
type Servers struct {
	Version int                   `json:"version"`
	Servers map[string]ServerSpec `json:"servers"`
}

// ServerSpec describes one MCP server. Either Command (stdio) or URL
// (Streamable HTTP) must be set; we intentionally don't support both.
type ServerSpec struct {
	Transport string            `json:"transport"`         // "stdio" | "http"
	Command   string            `json:"command,omitempty"` // stdio
	Args      []string          `json:"args,omitempty"`    // stdio
	Env       map[string]string `json:"env,omitempty"`     // stdio
	URL       string            `json:"url,omitempty"`     // http
	Headers   map[string]string `json:"headers,omitempty"` // http
}

// Validate checks that the spec describes a single, complete transport.
func (s ServerSpec) Validate(name string) error {
	switch s.Transport {
	case "stdio":
		if s.Command == "" {
			return fmt.Errorf("mcp: server %q: stdio transport requires command", name)
		}
		if s.URL != "" {
			return fmt.Errorf("mcp: server %q: stdio transport must not set url", name)
		}
	case "http":
		if s.URL == "" {
			return fmt.Errorf("mcp: server %q: http transport requires url", name)
		}
		if s.Command != "" {
			return fmt.Errorf("mcp: server %q: http transport must not set command", name)
		}
	case "":
		return fmt.Errorf("mcp: server %q: transport is required (\"stdio\" or \"http\")", name)
	default:
		return fmt.Errorf("mcp: server %q: unknown transport %q", name, s.Transport)
	}
	return nil
}

// Load reads <agentsDir>/mcp.json. A missing file is treated as
// "no servers configured" — not an error, since most projects never
// declare MCP servers.
func Load(agentsDir string) (Servers, error) {
	if agentsDir == "" {
		return Servers{}, nil
	}
	path := filepath.Join(agentsDir, MCPFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Servers{}, nil
		}
		return Servers{}, fmt.Errorf("mcp: read %s: %w", path, err)
	}
	var s Servers
	if err := json.Unmarshal(data, &s); err != nil {
		return Servers{}, fmt.Errorf("mcp: parse %s: %w", path, err)
	}
	for name, spec := range s.Servers {
		if err := spec.Validate(name); err != nil {
			return Servers{}, err
		}
	}
	return s, nil
}

// envInterpRe matches ${env:NAME} placeholders in env values and
// HTTP header values.
var envInterpRe = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// InterpolateEnv replaces ${env:NAME} placeholders in s by looking
// each NAME up via os.Getenv. Empty unset values pass through as
// empty strings — same semantics shells use.
func InterpolateEnv(s string) string {
	return envInterpRe.ReplaceAllStringFunc(s, func(match string) string {
		// Submatch group is the variable name.
		sub := envInterpRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return ""
		}
		return os.Getenv(sub[1])
	})
}

// InterpolateMap returns a copy of m with each value run through
// InterpolateEnv. Used for ServerSpec.Env and ServerSpec.Headers.
func InterpolateMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = InterpolateEnv(v)
	}
	return out
}
