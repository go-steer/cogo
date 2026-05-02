// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

// Package models is Cogo's adapter layer between cogo-side configuration
// and concrete LLM backends. The Provider interface keeps the rest of the
// codebase free of provider-specific imports so additional backends can be
// added later behind the same contract.
//
// In Slice 1 only the Gemini family (public Gemini API + Vertex AI) is
// implemented, both via the Go ADK's google.golang.org/adk/model/gemini
// package. Future providers (Anthropic, OpenAI, local Ollama) plug in here.
package models

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/adk/model"

	"github.com/go-steer/cogo/internal/config"
)

// Provider constructs concrete model.LLM instances on demand. A Provider
// is bound to one credential source (API key, Vertex project, etc.) at
// construction time; callers ask for specific models by ID through Model.
type Provider interface {
	// Name reports the provider identity ("gemini" or "vertex"). Used for
	// telemetry and diagnostic messages.
	Name() string

	// Model returns a usable model.LLM for the given model ID. The same
	// Provider may be asked for several models over its lifetime.
	Model(ctx context.Context, modelID string) (model.LLM, error)
}

// Constructor builds a Provider from validated config. Tests register
// alternates via Register so resolution stays decoupled from the imports
// of any single backend.
type Constructor func(*config.Config) (Provider, error)

var registry = map[string]Constructor{}

// Register installs a Constructor under its provider name. Idiomatically
// called from package init() in each backend implementation.
func Register(name string, c Constructor) {
	registry[name] = c
}

// Resolve picks the right Provider for cfg, honoring (in order): explicit
// cfg.Model.Provider, then env-based auto-detection per FR-3.3. Returns
// a clear error when no path is viable so the user knows which env var
// or config field to set.
func Resolve(cfg *config.Config) (Provider, error) {
	name := cfg.Model.Provider
	if name == "" {
		name = autoDetectProvider()
	}
	if name == "" {
		return nil, fmt.Errorf("models: no provider configured and none could be auto-detected; set model.provider in .agents/config.json or one of GOOGLE_API_KEY, GOOGLE_GENAI_USE_VERTEXAI=true (with GOOGLE_CLOUD_PROJECT)")
	}
	c, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("models: unknown provider %q (registered: %v)", name, registeredNames())
	}
	return c(cfg)
}

// autoDetectProvider implements the precedence in REQUIREMENTS FR-3.3.
func autoDetectProvider() string {
	if os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") == "true" && os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return config.ProviderVertex
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return config.ProviderGemini
	}
	return ""
}

func registeredNames() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
