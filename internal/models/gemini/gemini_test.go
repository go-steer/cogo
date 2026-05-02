// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package gemini

import (
	"strings"
	"testing"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
)

func TestResolve_ExplicitGemini_NoKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")

	cfg := config.DefaultConfig()
	cfg.Model.Provider = config.ProviderGemini
	_, err := models.Resolve(cfg)
	if err == nil || !strings.Contains(err.Error(), "api key is required") {
		t.Fatalf("expected api key error, got %v", err)
	}
}

func TestResolve_ExplicitGemini_WithKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")
	cfg := config.DefaultConfig()
	cfg.Model.Provider = config.ProviderGemini
	p, err := models.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name() != config.ProviderGemini {
		t.Errorf("provider name = %q, want %q", p.Name(), config.ProviderGemini)
	}
}

func TestResolve_ExplicitVertex_MissingProject(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_CLOUD_LOCATION", "")
	cfg := config.DefaultConfig()
	cfg.Model.Provider = config.ProviderVertex
	_, err := models.Resolve(cfg)
	if err == nil || !strings.Contains(err.Error(), "project and location are required") {
		t.Fatalf("expected vertex creds error, got %v", err)
	}
}

func TestResolve_ExplicitVertex_FromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = config.ProviderVertex
	cfg.Model.Vertex = &config.VertexConfig{Project: "p", Location: "us-central1"}
	p, err := models.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name() != config.ProviderVertex {
		t.Errorf("provider name = %q, want %q", p.Name(), config.ProviderVertex)
	}
}

func TestResolve_AutoDetectVertex(t *testing.T) {
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "p")
	t.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := config.DefaultConfig() // Provider unset → auto-detect
	p, err := models.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name() != config.ProviderVertex {
		t.Errorf("auto-detect picked %q, want vertex", p.Name())
	}
}

func TestResolve_AutoDetectGemini(t *testing.T) {
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_API_KEY", "k")

	cfg := config.DefaultConfig()
	p, err := models.Resolve(cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name() != config.ProviderGemini {
		t.Errorf("auto-detect picked %q, want gemini", p.Name())
	}
}

func TestResolve_NoProviderNoEnv(t *testing.T) {
	t.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := config.DefaultConfig()
	_, err := models.Resolve(cfg)
	if err == nil || !strings.Contains(err.Error(), "auto-detected") {
		t.Fatalf("expected auto-detect failure, got %v", err)
	}
}
