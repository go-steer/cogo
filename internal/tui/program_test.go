package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/go-steer/cogo/internal/agent"
	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/permissions"
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
	m := NewModel(cfg, a, "dark")
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	m.SetProgram(tm.GetProgram())
	return tm
}

// newTestModelExposed returns the model alongside the test program so
// individual tests can poke its public fields (e.g. AlwaysAllow) and
// inject permission requests directly without needing a real tool.
func newTestModelExposed(t *testing.T, script []testutil.ScriptedResponse) (*Model, *teatest.TestModel) {
	t.Helper()
	cfg := config.DefaultConfig()
	fake := &testutil.FakeModel{ModelName: "fake", Script: script}
	a, err := agent.New(fake)
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}
	m := NewModel(cfg, a, "dark")
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	m.SetProgram(tm.GetProgram())
	return m, tm
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

func TestProgram_SlashPaletteOpensAndExecutes(t *testing.T) {
	t.Parallel()
	tm := newTestModel(t, nil)

	tm.Type("/")
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("Slash commands")) && bytes.Contains(o, []byte("/help"))
	}, teatest.WithDuration(2*time.Second))

	// Down to /clear, Enter triggers /clear flow.
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("Clear chat history?"))
	}, teatest.WithDuration(2*time.Second))

	// Cancel the clear confirmation by typing "no" + enter, then exit.
	tm.Type("no")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestProgram_FilePaletteFromAt(t *testing.T) {
	t.Parallel()
	// Move into a temp dir so listProjectFiles has a small known set.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.md"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })

	tm := newTestModel(t, nil)

	// Trigger @-palette.
	tm.Type("look at @")
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("Files (")) &&
			bytes.Contains(o, []byte("alpha.md")) && bytes.Contains(o, []byte("beta.md"))
	}, teatest.WithDuration(2*time.Second))

	// Filter to just alpha.
	tm.Type("alp")
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("alpha.md")) && !bytes.Contains(o, []byte("beta.md"))
	}, teatest.WithDuration(2*time.Second))

	// Enter inserts the @-path and closes the palette (no submission).
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("@alpha.md ")) && !bytes.Contains(o, []byte("Files ("))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestProgram_PermissionModalApprovesAndDenies(t *testing.T) {
	t.Parallel()
	_, tm := newTestModelExposed(t, nil)

	// Inject a permission request as if a tool handler had asked.
	out := make(chan permissions.Decision, 1)
	tm.Send(confirmReqMsg{
		Req: permissions.PromptRequest{
			Kind:        permissions.PromptKindBash,
			ToolName:    "bash",
			Detail:      "git push origin main",
			PersistTool: "bash",
			PersistKey:  "git push origin main",
		},
		Out: out,
	})

	// Modal should appear in the rendered output.
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("git push origin main")) &&
			bytes.Contains(o, []byte("[y] allow once"))
	}, teatest.WithDuration(2*time.Second))

	// Approve once.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if got := <-out; got != permissions.DecisionAllowOnce {
		t.Errorf("decision = %v, want allow-once", got)
	}

	// Echo line should land in the chat history.
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("Permission allow-once: bash"))
	}, teatest.WithDuration(2*time.Second))

	// Now deny a second request.
	out2 := make(chan permissions.Decision, 1)
	tm.Send(confirmReqMsg{
		Req: permissions.PromptRequest{
			Kind:     permissions.PromptKindBash,
			ToolName: "bash",
			Detail:   "rm important.txt",
		},
		Out: out2,
	})
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("rm important.txt"))
	}, teatest.WithDuration(2*time.Second))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if got := <-out2; got != permissions.DecisionDeny {
		t.Errorf("decision = %v, want deny", got)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestProgram_PermissionModalAlwaysCallsHook(t *testing.T) {
	t.Parallel()
	m, tm := newTestModelExposed(t, nil)

	var got *permissions.PromptRequest
	m.AlwaysAllow = func(req permissions.PromptRequest) error {
		got = &req
		return nil
	}

	out := make(chan permissions.Decision, 1)
	tm.Send(confirmReqMsg{
		Req: permissions.PromptRequest{
			Kind:        permissions.PromptKindPathScope,
			ToolName:    "read_file",
			Detail:      "read /var/log/x.log (out of scope)",
			PersistTool: "path_scope",
			PersistKey:  "/var/log",
		},
		Out: out,
	})
	teatest.WaitFor(t, tm.Output(), func(o []byte) bool {
		return bytes.Contains(o, []byte("/var/log/x.log"))
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if d := <-out; d != permissions.DecisionAllowAlways {
		t.Errorf("decision = %v, want allow-always", d)
	}
	// Persistence hook should have fired.
	if got == nil || got.PersistKey != "/var/log" {
		t.Errorf("AlwaysAllow hook not called with expected req: %+v", got)
	}

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
