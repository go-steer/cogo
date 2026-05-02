// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"strings"
	"testing"

	"google.golang.org/adk/tool"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/permissions"
)

func TestBash_RunsAndCapturesOutput(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	gate := permissions.New(permissions.Options{Mode: permissions.ModeYolo})
	fn := bashFunc(gate, cfg)
	res, err := fn(tool.Context(nil), bashArgs{Command: "printf hello"})
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d, want 0", res.ExitCode)
	}
	if res.Stdout != "hello" {
		t.Errorf("stdout = %q", res.Stdout)
	}
}

func TestBash_RefusesDenylist(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	gate := permissions.New(permissions.Options{Mode: permissions.ModeYolo}) // even yolo
	fn := bashFunc(gate, cfg)
	_, err := fn(tool.Context(nil), bashArgs{Command: "rm -rf /"})
	if err == nil || !strings.Contains(err.Error(), "filesystem root") {
		t.Errorf("expected denylist refusal, got %v", err)
	}
}

func TestBash_TimesOut(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	gate := permissions.New(permissions.Options{Mode: permissions.ModeYolo})
	fn := bashFunc(gate, cfg)
	_, err := fn(tool.Context(nil), bashArgs{Command: "sleep 5", TimeoutSeconds: 1})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout, got %v", err)
	}
}

func TestBash_NonzeroExitNotAnError(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	gate := permissions.New(permissions.Options{Mode: permissions.ModeYolo})
	fn := bashFunc(gate, cfg)
	res, err := fn(tool.Context(nil), bashArgs{Command: "false"})
	if err != nil {
		t.Errorf("non-zero exit should not be a Go error: %v", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("exit = %d, want 1", res.ExitCode)
	}
}
