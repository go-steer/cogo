// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

// Package telemetry initializes OpenTelemetry for the agent loop.
//
// Per REQUIREMENTS NFR-12 / DESIGN §3.12, telemetry is off by default
// — no exporter is configured — so a fresh `cogo` invocation makes
// zero outbound network calls. Users opt in by:
//
//   - --otel=console (writes spans to stderr; useful for local debug)
//   - cfg.OTEL.Exporter = "otlp" + standard OTEL env vars
//     (OTEL_EXPORTER_OTLP_ENDPOINT, etc.) to ship to a collector
//   - cfg.OTEL.Exporter = "none" (the default; no spans leave)
//
// Spike finding: ADK's telemetry.New constructs providers but does
// NOT install them as OTEL globals; you must call SetGlobalOtelProviders
// explicitly or ADK's instrumentation will run against the noop tracer.
// This package handles that.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	adktelemetry "google.golang.org/adk/telemetry"
)

// Mode names recognized by Setup. They map to cfg.OTEL.Exporter.
const (
	ModeNone    = "none"    // default; no spans exported
	ModeConsole = "console" // stdout exporter; for local dev
	ModeOTLP    = "otlp"    // honors OTEL_EXPORTER_OTLP_ENDPOINT etc.
)

// Setup configures OpenTelemetry. Returns a shutdown function the
// caller MUST call (typically deferred) so buffered spans get flushed.
//
// When mode is "" or "none", no providers are constructed and the
// shutdown returns nil — call sites stay clean either way.
func Setup(ctx context.Context, mode string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }
	switch mode {
	case "", ModeNone:
		return noop, nil
	case ModeConsole, ModeOTLP:
		// fall through
	default:
		return noop, fmt.Errorf("telemetry: unknown mode %q (want console/otlp/none)", mode)
	}

	var opts []adktelemetry.Option
	if mode == ModeConsole {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return noop, fmt.Errorf("telemetry: console exporter: %w", err)
		}
		opts = append(opts, adktelemetry.WithSpanProcessors(sdktrace.NewBatchSpanProcessor(exp)))
	}
	// For mode==otlp we let ADK's telemetry.New honor the standard
	// OTEL_EXPORTER_OTLP_* env vars without explicit option overrides.

	providers, err := adktelemetry.New(ctx, opts...)
	if err != nil {
		return noop, fmt.Errorf("telemetry: init: %w", err)
	}
	providers.SetGlobalOtelProviders()
	return providers.Shutdown, nil
}
