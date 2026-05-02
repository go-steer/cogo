// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-steer/cogo/internal/config"
)

func TestWriteAgentsDir_Defaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteAgentsDir(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, AgentsDirName, config.ConfigFileName)
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var got config.Config
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("config didn't parse: %v\n%s", err, body)
	}
	if got.Model.Name == "" {
		t.Errorf("model.name should be populated by default")
	}
	if _, err := os.Stat(filepath.Join(dir, AgentsDirName, ".gitignore")); err != nil {
		t.Errorf(".gitignore missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, AgentsDirName, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md missing: %v", err)
	}
}

func TestWriteAgentsDir_RefusesExistingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteAgentsDir(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	err := WriteAgentsDir(dir, Options{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected refuse-overwrite error, got %v", err)
	}
}

func TestWriteAgentsDir_ForceOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteAgentsDir(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Model.Name = "gemini-3-flash-preview"
	if err := WriteAgentsDir(dir, Options{Cfg: cfg, Force: true}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, AgentsDirName, config.ConfigFileName))
	if !strings.Contains(string(body), "gemini-3-flash-preview") {
		t.Errorf("force did not persist new config:\n%s", body)
	}
}

func TestWriteAgentsDir_PreservesAGENTS_MD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Pre-existing AGENTS.md user already authored.
	if err := os.MkdirAll(filepath.Join(dir, AgentsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "MY OWN MEMORY"
	if err := os.WriteFile(filepath.Join(dir, AgentsDirName, "AGENTS.md"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentsDir(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, AgentsDirName, "AGENTS.md"))
	if string(body) != custom {
		t.Errorf("custom AGENTS.md was overwritten:\n%s", body)
	}
}
