// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ElicitorFn is the host-supplied bridge that turns a server's
// elicitation request into a user response. The TUI plugs in a function
// that opens a modal and blocks; the headless path leaves it nil and
// falls back to DeclineHandler.
//
// The implementation must respect ctx — if it returns ctx.Err the SDK
// translates that into a protocol-level cancel.
type ElicitorFn func(ctx context.Context, serverName string, req *mcp.ElicitRequest) (*mcp.ElicitResult, error)

// handlerFor picks the right elicitation handler for a server. When
// elicitor is non-nil we route through it (the interactive TUI);
// otherwise fall back to the decline-with-notice stub for headless
// runs and tests.
func handlerFor(serverName string, send func(string), elicitor ElicitorFn) func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	if elicitor != nil {
		return func(ctx context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return elicitor(ctx, serverName, req)
		}
	}
	return DeclineHandler(serverName, send)
}

// DeclineHandler returns an MCP elicitation handler that declines every
// request and emits a one-line notice through send (typically stderr in
// headless mode).
//
// Used as the fallback when no interactive elicitor is wired. Keeps
// Cogo from hanging when an MCP server tries to elicit on a path with
// no UI to render.
func DeclineHandler(serverName string, send func(string)) func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	return func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		message := elicitationMessage(req)
		if send != nil {
			send(fmt.Sprintf("MCP server %q requested elicitation (%s); declined (no interactive UI)", serverName, message))
		}
		return &mcp.ElicitResult{Action: "decline"}, nil
	}
}

// elicitationMessage renders a short one-line description of the
// request: typically the message the server attached to the prompt,
// falling back to a schema summary if missing.
func elicitationMessage(req *mcp.ElicitRequest) string {
	if req == nil || req.Params == nil {
		return "no details"
	}
	if msg := req.Params.Message; msg != "" {
		return "message: " + msg
	}
	return "schema-driven request"
}
