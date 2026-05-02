package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/go-steer/cogo/internal/testutil"
)

func TestNew_NilModel(t *testing.T) {
	t.Parallel()
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil model")
	}
}

func TestRun_ConcatenatesFinalText(t *testing.T) {
	t.Parallel()
	model := &testutil.FakeModel{
		ModelName: "fake-model",
		Script: []testutil.ScriptedResponse{
			{TextChunks: []string{"Hello, ", "world!"}},
		},
	}
	a, err := New(model, WithName("test_agent"), WithInstruction("be brief"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var partials, completes int
	var assembled strings.Builder
	for event, err := range a.Run(context.Background(), "ping") {
		if err != nil {
			t.Fatalf("event err: %v", err)
		}
		if event.Partial {
			partials++
		}
		if event.TurnComplete {
			completes++
		}
		if event.Content != nil && !event.Partial {
			for _, p := range event.Content.Parts {
				if p.Text != "" {
					assembled.WriteString(p.Text)
				}
			}
		}
	}

	if got := assembled.String(); got != "Hello, world!" {
		t.Errorf("final text = %q, want %q", got, "Hello, world!")
	}
	if partials < 1 {
		t.Errorf("expected at least one Partial event with StreamingModeSSE, got 0")
	}
	if completes < 1 {
		t.Errorf("expected at least one TurnComplete event, got 0")
	}
}

func TestRun_FakeModelCallCount(t *testing.T) {
	t.Parallel()
	model := &testutil.FakeModel{
		ModelName: "fake-model",
		Script:    []testutil.ScriptedResponse{{TextChunks: []string{"hi"}}},
	}
	a, err := New(model)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, err := range a.Run(context.Background(), "ping") {
		if err != nil {
			t.Fatalf("event err: %v", err)
		}
	}
	if got := model.Calls(); got < 1 {
		t.Errorf("model.Calls() = %d, want >= 1", got)
	}
}
