// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package headless

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-steer/cogo/internal/testutil"
	"github.com/go-steer/cogo/internal/usage"
)

func TestRun_StreamsPartialsToStdout(t *testing.T) {
	t.Parallel()
	model := &testutil.FakeModel{
		ModelName: "fake",
		Script: []testutil.ScriptedResponse{
			{TextChunks: []string{"The ", "answer ", "is 4."}},
		},
	}

	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), model, "what is 2+2", &stdout, &stderr, nil, usage.Pricing{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != ExitOK {
		t.Errorf("exit code = %d, want %d", code, ExitOK)
	}
	if got := stdout.String(); got != "The answer is 4.\n" {
		t.Errorf("stdout = %q, want %q", got, "The answer is 4.\n")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty in Slice 1 (no tool summaries), got %q", stderr.String())
	}
}

func TestRun_EmptyPromptIsConfigError(t *testing.T) {
	t.Parallel()
	model := &testutil.FakeModel{}
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), model, "", &stdout, &stderr, nil, usage.Pricing{})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt-required error, got %v", err)
	}
	if code != ExitConfigError {
		t.Errorf("exit code = %d, want %d", code, ExitConfigError)
	}
}

func TestRun_NoFinalNewlineWhenSilent(t *testing.T) {
	t.Parallel()
	// FakeModel with a script entry that has no text chunks → silent.
	model := &testutil.FakeModel{
		Script: []testutil.ScriptedResponse{{}},
	}
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), model, "ping", &stdout, &stderr, nil, usage.Pricing{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != ExitOK {
		t.Errorf("exit code = %d, want %d", code, ExitOK)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}
