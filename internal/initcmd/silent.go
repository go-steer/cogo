// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

// Package initcmd implements the `cogo init` subcommand: scaffolding
// .agents/{config.json, .gitignore, AGENTS.md} for a fresh project.
//
// Two modes:
//   - silent (default): drop sensible defaults with no prompts
//   - --interactive: walk a Bubble Tea wizard for provider, model,
//     and permission mode (see wizard.go).
//
// The silent path is what `cogo init` runs; the wizard returns its
// final config to the caller, which then writes via WriteAgentsDir.
package initcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-steer/cogo/internal/config"
)

// AgentsDirName mirrors config.AgentsDirName so call sites don't need
// to import both packages.
const AgentsDirName = config.AgentsDirName

// Options controls how WriteAgentsDir behaves.
type Options struct {
	// Cfg is the configuration to write. When nil, config.DefaultConfig() is used.
	Cfg *config.Config
	// Force overwrites an existing .agents/config.json. Without it, an
	// existing config produces a clear "already initialized" error.
	Force bool
	// Memory is an optional starter AGENTS.md body. Empty means "leave
	// AGENTS.md as a placeholder so the user can fill it in."
	Memory string
}

// WriteAgentsDir creates targetDir/.agents/ and populates it with
// config.json, .gitignore, and AGENTS.md.
func WriteAgentsDir(targetDir string, opts Options) error {
	if targetDir == "" {
		return errors.New("init: target directory is required")
	}
	if opts.Cfg == nil {
		opts.Cfg = config.DefaultConfig()
	}
	if err := opts.Cfg.Validate(); err != nil {
		return fmt.Errorf("init: validate config: %w", err)
	}

	agentsDir := filepath.Join(targetDir, AgentsDirName)
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("init: mkdir %s: %w", agentsDir, err)
	}

	configPath := filepath.Join(agentsDir, config.ConfigFileName)
	if !opts.Force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("init: %s already exists; pass --force to overwrite", configPath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("init: stat config: %w", err)
		}
	}

	if err := config.Save(configPath, opts.Cfg); err != nil {
		return fmt.Errorf("init: write config: %w", err)
	}

	gitignorePath := filepath.Join(agentsDir, ".gitignore")
	if err := writeIfMissing(gitignorePath, defaultGitignore, opts.Force); err != nil {
		return err
	}

	memoryPath := filepath.Join(agentsDir, "AGENTS.md")
	body := opts.Memory
	if body == "" {
		body = defaultMemoryStub
	}
	if err := writeIfMissing(memoryPath, body, opts.Force); err != nil {
		return err
	}

	return nil
}

// writeIfMissing writes content to path only when path doesn't exist
// (or when force=true). Files the user has already authored are
// preserved.
func writeIfMissing(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("init: stat %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("init: write %s: %w", path, err)
	}
	return nil
}

// defaultGitignore for .agents/. Matches the project-root .gitignore
// strategy: per-user state stays local, the rest is committable.
const defaultGitignore = `# Per-user state — keep these out of version control.
sessions/
logs/
`

const defaultMemoryStub = `# Project memory

This file is loaded into Cogo's system prompt at startup. Use it to
capture conventions, preferences, and context that the agent should
always have available.

Examples to write:
- "We use the Foo framework. Prefer functional style over OOP."
- "Tests live in ` + "`./test`" + `; run with ` + "`go test ./...`" + `."
- "Don't suggest changes under ` + "`vendor/`" + ` — that's generated."

Replace this stub with whatever's true for your project.
`
