// Package gemini implements models.Provider for the Gemini family,
// covering both the public Gemini API (API-key auth) and Vertex AI
// (Application Default Credentials + GCP project).
//
// The two are exposed as distinct provider names ("gemini" and "vertex")
// so users and automation can pin to a backend explicitly. Both delegate
// to google.golang.org/adk/model/gemini under the hood.
package gemini

import (
	"context"
	"fmt"
	"os"

	adkmodel "google.golang.org/adk/model"
	adkgemini "google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/models"
)

func init() {
	models.Register(config.ProviderGemini, newGeminiAPI)
	models.Register(config.ProviderVertex, newVertexAI)
}

// Provider is the Gemini-family implementation of models.Provider.
//
// Tests can construct one directly via NewAPIKey or NewVertex; production
// code should go through models.Resolve so config-driven auto-detection
// is honored.
type Provider struct {
	name   string
	cfg    *genai.ClientConfig
	prefix string // for diagnostic messages
}

// Name reports the provider identity (e.g. "gemini" or "vertex").
func (p *Provider) Name() string { return p.name }

// Model constructs a model.LLM for the given model ID.
func (p *Provider) Model(ctx context.Context, modelID string) (adkmodel.LLM, error) {
	if modelID == "" {
		return nil, fmt.Errorf("%s: model id is required", p.prefix)
	}
	llm, err := adkgemini.NewModel(ctx, modelID, p.cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: new model %q: %w", p.prefix, modelID, err)
	}
	return llm, nil
}

// NewAPIKey returns a Provider authenticated against the public Gemini API
// using key. Empty key is rejected so the failure mode is clear at startup.
func NewAPIKey(key string) (*Provider, error) {
	if key == "" {
		return nil, fmt.Errorf("gemini: api key is required (set GOOGLE_API_KEY or model.api_key in .agents/config.json)")
	}
	return &Provider{
		name:   config.ProviderGemini,
		prefix: "gemini",
		cfg: &genai.ClientConfig{
			APIKey:  key,
			Backend: genai.BackendGeminiAPI,
		},
	}, nil
}

// NewVertex returns a Provider authenticated against Vertex AI for the
// given GCP project and location, using Application Default Credentials.
func NewVertex(project, location string) (*Provider, error) {
	if project == "" || location == "" {
		return nil, fmt.Errorf("vertex: project and location are required (set model.vertex.{project,location} in .agents/config.json or GOOGLE_CLOUD_PROJECT / GOOGLE_CLOUD_LOCATION env vars)")
	}
	return &Provider{
		name:   config.ProviderVertex,
		prefix: "vertex",
		cfg: &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  project,
			Location: location,
		},
	}, nil
}

// newGeminiAPI is the registry constructor for "gemini". Resolves the API
// key from cfg.Model.APIKey (precedence) then GOOGLE_API_KEY.
func newGeminiAPI(cfg *config.Config) (models.Provider, error) {
	key := cfg.Model.APIKey
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	return NewAPIKey(key)
}

// newVertexAI is the registry constructor for "vertex". Project/location
// resolution: cfg.Model.Vertex (precedence) then GOOGLE_CLOUD_PROJECT /
// GOOGLE_CLOUD_LOCATION.
func newVertexAI(cfg *config.Config) (models.Provider, error) {
	project, location := "", ""
	if cfg.Model.Vertex != nil {
		project = cfg.Model.Vertex.Project
		location = cfg.Model.Vertex.Location
	}
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_LOCATION")
	}
	return NewVertex(project, location)
}
