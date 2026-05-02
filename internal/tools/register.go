// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/permissions"
)

// Registry is the assembled built-in tool set returned to the agent.
//
// Slice 3 ships file I/O, bash, and todo. Web tools, glob/grep, and
// the subagent tool follow in later slices.
type Registry struct {
	Tools []tool.Tool
	Todo  *TodoStore // exposed so callers can inspect plan progress
}

// Build constructs the registry. cfg supplies output-truncation caps;
// gate gates every mutating call.
//
// We deliberately do NOT set ADK's functiontool.Config.RequireConfirmation
// even when the gate is in "ask" mode. Cogo's gate handles approval
// itself by calling its Prompter from inside each tool handler — the
// handler blocks until the user responds. Going through ADK's HITL
// flow (LongRunningToolIDs + state injection) would be a second
// approval round-trip on top of ours.
func Build(cfg *config.Config, gate *permissions.Gate) (*Registry, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tools: cfg is required")
	}
	if gate == nil {
		return nil, fmt.Errorf("tools: gate is required")
	}
	store := NewTodoStore()

	specs := []struct {
		name string
		desc string
		ctor func() (tool.Tool, error)
	}{
		{"read_file", "Read a file from disk and return its contents.", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "read_file", Description: "Read a file from disk. Honors offset/limit for large files.",
			}, readFileFunc(gate, cfg))
		}},
		{"write_file", "Write or overwrite a file with the given content.", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "write_file", Description: "Create or overwrite a file. Asks for confirmation in 'ask' mode.",
			}, writeFileFunc(gate))
		}},
		{"edit_file", "Replace one occurrence of an exact string in a file.", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "edit_file", Description: "Replace exactly one occurrence of old_string with new_string in path.",
			}, editFileFunc(gate))
		}},
		{"list_dir", "List entries of a directory.", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "list_dir", Description: "List the entries (files and subdirectories) of a directory.",
			}, listDirFunc(gate, cfg))
		}},
		{"bash", "Run a shell command and return its output.", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "bash", Description: "Execute a shell command via /bin/sh -c with a timeout.",
			}, bashFunc(gate, cfg))
		}},
		{"todo", "Maintain an agent-facing todo list (list/add/set_status/clear).", func() (tool.Tool, error) {
			return functiontool.New(functiontool.Config{
				Name: "todo", Description: "Maintain a short todo list visible to the user. Actions: list, add, set_status, clear.",
			}, todoFunc(store))
		}},
	}

	out := &Registry{Todo: store}
	for _, s := range specs {
		t, err := s.ctor()
		if err != nil {
			return nil, fmt.Errorf("tools: build %s: %w", s.name, err)
		}
		out.Tools = append(out.Tools, t)
	}
	return out, nil
}
