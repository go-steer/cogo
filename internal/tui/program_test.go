package tui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/testutil"
)

// newTestModel constructs a TUI model wired to a FakeModel-backed agent
// and a teatest.TestModel. The TestModel's underlying *tea.Program is
// installed as the Model's programSender so the agent goroutine can
// Send streamed events back into the same loop.
func newTestModel(t *testing.T, script []testutil.ScriptedResponse) *teatest.TestModel {
	t.Helper()
	cfg := config.DefaultConfig()
	fake := &testutil.FakeModel{ModelName: "fake", Script: script}
	a, err := agent.New(fake)
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}
	m := NewModel(cfg, a)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	m.SetProgram(tm.GetProgram())
	return tm
}

func TestProgram_StreamsThenRendersAndQuits(t *testing.T) {
	t.Parallel()
	tm := newTestModel(t, []testutil.ScriptedResponse{
		{TextChunks: []string{"Hello, ", "world!"}},
	})

	tm.Type("ping")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Hello, world!"))
	}, teatest.WithDuration(3*time.Second))

	// Quit cleanly via /quit.
	tm.Type("/quit")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestProgram_HelpCommandShowsHelp(t *testing.T) {
	t.Parallel()
	tm := newTestModel(t, nil)

	tm.Type("/help")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Slash commands:"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestProgram_UnknownSlashShowsHint(t *testing.T) {
	t.Parallel()
	tm := newTestModel(t, nil)

	tm.Type("/whatever")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Unknown command"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestProgram_ClearAsksForConfirmation(t *testing.T) {
	t.Parallel()
	// Use a non-empty script so the first turn produces visible output we
	// can wait on, ensuring state is back to idle before we issue /clear.
	tm := newTestModel(t, []testutil.ScriptedResponse{
		{TextChunks: []string{"acknowledged"}},
	})

	tm.Type("hi")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("acknowledged"))
	}, teatest.WithDuration(3*time.Second))

	tm.Type("/clear")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Clear chat history?"))
	}, teatest.WithDuration(2*time.Second))

	// Confirm.
	tm.Type("yes")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("History cleared."))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
