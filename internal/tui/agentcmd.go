package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-steer/cogo/internal/agent"
)

// programSender is the subset of *tea.Program the goroutine uses. We
// take this as an interface so tests can inject a fake without spinning
// up a real program.
type programSender interface {
	Send(tea.Msg)
}

// startAgentTurn launches an agent turn in a goroutine. It returns
// immediately; subsequent events (streamChunkMsg, turnDoneMsg, turnErrMsg,
// turnCancelledMsg) flow through send.Send.
//
// ctx controls the turn — cancelling it interrupts the model and tools
// mid-stream and the goroutine exits with turnCancelledMsg.
func startAgentTurn(ctx context.Context, send programSender, a *agent.Agent, prompt string) {
	go func() {
		for event, err := range a.Run(ctx, prompt) {
			if err != nil {
				if ctx.Err() != nil {
					send.Send(turnCancelledMsg{})
				} else {
					send.Send(turnErrMsg{Err: err})
				}
				return
			}
			if event.Content == nil || !event.Partial {
				continue
			}
			for _, p := range event.Content.Parts {
				if p.Text != "" {
					send.Send(streamChunkMsg{Text: p.Text})
				}
			}
		}
		// Iterator drained without error; check whether ctx was cancelled
		// at the very end (rare but possible after the last event).
		if ctx.Err() != nil {
			send.Send(turnCancelledMsg{})
			return
		}
		send.Send(turnDoneMsg{})
	}()
}
