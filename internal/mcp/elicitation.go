package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DeclineHandler returns an MCP elicitation handler that declines every
// request and emits a one-line notice through send (typically the TUI's
// system-message append, or stderr in headless).
//
// Slice 4b stub: a real schema-driven form modal is post-V1 polish.
// Until then, this keeps Cogo from hanging when an MCP server tries to
// elicit, and keeps the user informed that the server asked something
// it didn't get.
func DeclineHandler(serverName string, send func(string)) func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	return func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		message := elicitationMessage(req)
		if send != nil {
			send(fmt.Sprintf("MCP server %q requested elicitation (%s); declined for now (full UX coming in a later release)", serverName, message))
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
