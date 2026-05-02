// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

// Package usage tracks token + cost accounting for the agent loop.
//
// Every model call returns a UsageMetadata block with input and output
// token counts; a Tracker accumulates these across a session. Pricing
// numbers come from a built-in table that callers may override per
// model via .agents/config.json (model.pricing).
package usage

import (
	"strings"

	"github.com/go-steer/cogo/internal/config"
)

// Pricing is the per-million-token rate for one model. Both fields are
// in USD and apply to a single direction (input or output).
type Pricing struct {
	InputPerMTok  float64 // USD per 1,000,000 input tokens
	OutputPerMTok float64 // USD per 1,000,000 output tokens
}

// IsZero reports whether neither rate is set (we don't know how to
// price this model).
func (p Pricing) IsZero() bool { return p.InputPerMTok == 0 && p.OutputPerMTok == 0 }

// builtinPricing holds Cogo's defaults for the Gemini 3.x preview
// lineup. Numbers are placeholders modeled on Gemini 2.5 list prices
// (per Google's pricing page at time of writing); update when Google
// publishes 3.x rates. Users override via cfg.Model.Pricing.
//
// Keys are matched case-insensitively. A prefix match (modelID starts
// with key) is also accepted so date-suffixed variants like
// "gemini-3.1-pro-preview-05-15" still get reasonable rates.
var builtinPricing = map[string]Pricing{
	"gemini-3.1-pro-preview":         {InputPerMTok: 1.25, OutputPerMTok: 5.00},
	"gemini-3.1-pro":                 {InputPerMTok: 1.25, OutputPerMTok: 5.00},
	"gemini-3-flash-preview":         {InputPerMTok: 0.075, OutputPerMTok: 0.30},
	"gemini-3-flash":                 {InputPerMTok: 0.075, OutputPerMTok: 0.30},
	"gemini-3.1-flash-lite-preview":  {InputPerMTok: 0.04, OutputPerMTok: 0.15},
	"gemini-3.1-flash-image-preview": {InputPerMTok: 0.10, OutputPerMTok: 0.40},
}

// PriceFor returns the Pricing for modelID. Resolution order:
//  1. Explicit cfg override (cfg.Model.Pricing) when modelID matches
//     cfg.Model.Name (case-insensitive).
//  2. Exact match in the built-in table.
//  3. Prefix match in the built-in table (longest first).
//  4. Zero pricing — caller should treat cost as unknown.
func PriceFor(modelID string, cfg *config.Config) Pricing {
	low := strings.ToLower(strings.TrimSpace(modelID))
	if cfg != nil && cfg.Model.Pricing != nil &&
		strings.EqualFold(cfg.Model.Name, modelID) &&
		(cfg.Model.Pricing.InputPerMTok > 0 || cfg.Model.Pricing.OutputPerMTok > 0) {
		return Pricing{
			InputPerMTok:  cfg.Model.Pricing.InputPerMTok,
			OutputPerMTok: cfg.Model.Pricing.OutputPerMTok,
		}
	}
	if p, ok := builtinPricing[low]; ok {
		return p
	}
	// Longest-prefix match.
	var best string
	for k := range builtinPricing {
		if strings.HasPrefix(low, k) && len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		return builtinPricing[best]
	}
	return Pricing{}
}

// CostUSD returns the dollar cost of (input, output) tokens at p.
func (p Pricing) CostUSD(inputTokens, outputTokens int) float64 {
	const million = 1_000_000.0
	return (float64(inputTokens)/million)*p.InputPerMTok +
		(float64(outputTokens)/million)*p.OutputPerMTok
}
