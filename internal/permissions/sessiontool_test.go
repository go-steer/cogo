// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package permissions

import (
	"context"
	"testing"
)

// TestGate_DecisionAllowSessionTool_SuppressesFurtherPrompts pins the
// v0.1.3 contract: once the user picks DecisionAllowSessionTool on a
// prompt for tool X, every subsequent gate request for tool X (any
// args, any file, any path) MUST go through without prompting again
// until the session ends. The whole point of the feature is to stop
// re-prompting for read_file / ls / etc.
//
// DO NOT silence this test. A failure means the user clicked
// "[t] this tool · session", which exists only to suppress further
// prompts, and immediately got prompted again. That defeats the
// affordance entirely.
func TestGate_DecisionAllowSessionTool_SuppressesFurtherPrompts(t *testing.T) {
	t.Parallel()
	prompter := &fakePrompter{decision: DecisionAllowSessionTool}
	g := New(Options{Mode: ModeAsk, Prompter: prompter})

	ctx := context.Background()
	// First call prompts and the user picks "tool · session".
	if err := g.CheckGeneric(ctx, "read_file", "go.mod"); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if len(prompter.calls) != 1 {
		t.Fatalf("first call should prompt exactly once; got %d", len(prompter.calls))
	}

	// All subsequent read_file calls — different keys — must NOT prompt.
	for _, key := range []string{"go.sum", "internal/tui/model.go", "README.md", "anything-else"} {
		if err := g.CheckGeneric(ctx, "read_file", key); err != nil {
			t.Errorf("read_file %q after AllowSessionTool: unexpected error %v", key, err)
		}
	}
	if len(prompter.calls) != 1 {
		t.Errorf("subsequent read_file calls should be silent; prompter was called %d times total", len(prompter.calls))
	}

	// A different tool still prompts — AllowSessionTool only trusts
	// the tool that was approved, not every tool.
	if err := g.CheckGeneric(ctx, "bash", "ls -la"); err != nil {
		t.Errorf("bash call: unexpected error %v", err)
	}
	if len(prompter.calls) != 2 {
		t.Errorf("a different tool should still prompt; got total prompter.calls=%d, want 2", len(prompter.calls))
	}
}

// TestGate_FileReadWrite_SkipScopeCheckOnSessionTool covers the file-
// path branch. Once read_file (or write_file) is trusted tool-wide,
// even out-of-scope paths must go through without an additional path-
// scope prompt. Without this the user gets re-prompted on every
// out-of-scope file access despite already trusting the tool.
func TestGate_FileReadWrite_SkipScopeCheckOnSessionTool(t *testing.T) {
	t.Parallel()
	prompter := &fakePrompter{decision: DecisionAllowSessionTool}
	scope, err := NewPathScope("/tmp/in-scope", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	g := New(Options{Mode: ModeAsk, Scope: scope, Prompter: prompter})

	ctx := context.Background()
	// Out-of-scope read prompts the user once; they pick AllowSessionTool.
	if err := g.CheckFileRead(ctx, "read_file", "/etc/hosts"); err != nil {
		t.Fatalf("first out-of-scope read: unexpected error: %v", err)
	}
	if len(prompter.calls) != 1 {
		t.Fatalf("first out-of-scope read should prompt; got %d", len(prompter.calls))
	}

	// Subsequent reads of any path must not re-prompt.
	for _, p := range []string{"/etc/passwd", "/var/log/syslog", "/srv/secret"} {
		if err := g.CheckFileRead(ctx, "read_file", p); err != nil {
			t.Errorf("read_file %q after AllowSessionTool: unexpected error %v", p, err)
		}
	}
	if len(prompter.calls) != 1 {
		t.Errorf("post-AllowSessionTool reads should be silent; got total %d", len(prompter.calls))
	}
}
