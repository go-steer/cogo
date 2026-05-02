// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDeclineHandler_DeclinesAndNotifies(t *testing.T) {
	t.Parallel()
	var got string
	send := func(s string) { got = s }
	h := DeclineHandler("github", send)

	res, err := h(context.Background(), &mcpsdk.ElicitRequest{
		Params: &mcpsdk.ElicitParams{Message: "what's your token?"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != "decline" {
		t.Errorf("action = %q, want decline", res.Action)
	}
	if !strings.Contains(got, "github") || !strings.Contains(got, "what's your token?") {
		t.Errorf("notification missing detail: %q", got)
	}
}

func TestDeclineHandler_NilSendOK(t *testing.T) {
	t.Parallel()
	h := DeclineHandler("x", nil)
	if _, err := h(context.Background(), &mcpsdk.ElicitRequest{}); err != nil {
		t.Errorf("nil-send variant should be safe: %v", err)
	}
}
