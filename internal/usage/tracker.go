package usage

import (
	"sync"
	"time"
)

// Turn captures one model call's resource use. Times are wall clock so
// summary lines can include session duration without a monotonic ref.
type Turn struct {
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	At           time.Time
}

// Totals aggregates a slice of Turns.
type Totals struct {
	Turns        int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// Tracker accumulates per-turn usage for one Cogo session.
//
// Thread-safe: the agent goroutine (or headless run loop) calls
// Append; the TUI's render path reads via Last/Totals/All.
type Tracker struct {
	mu       sync.Mutex
	turns    []Turn
	startedAt time.Time
}

// NewTracker returns a tracker with its session-start time set to now.
func NewTracker() *Tracker { return &Tracker{startedAt: time.Now()} }

// Append records one turn's usage. Cost is computed via the supplied
// Pricing; pass a zero Pricing to skip cost tracking.
func (t *Tracker) Append(model string, inputTokens, outputTokens int, p Pricing) Turn {
	turn := Turn{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      p.CostUSD(inputTokens, outputTokens),
		At:           time.Now(),
	}
	t.mu.Lock()
	t.turns = append(t.turns, turn)
	t.mu.Unlock()
	return turn
}

// Last returns the most recently appended turn, or zero if none yet.
func (t *Tracker) Last() (Turn, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.turns) == 0 {
		return Turn{}, false
	}
	return t.turns[len(t.turns)-1], true
}

// Totals returns the cumulative usage across all turns.
func (t *Tracker) Totals() Totals {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := Totals{Turns: len(t.turns)}
	for _, x := range t.turns {
		out.InputTokens += x.InputTokens
		out.OutputTokens += x.OutputTokens
		out.CostUSD += x.CostUSD
	}
	return out
}

// All returns a copy of every recorded turn (for /stats rendering).
func (t *Tracker) All() []Turn {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Turn, len(t.turns))
	copy(out, t.turns)
	return out
}

// Duration reports wall-clock time since NewTracker was called.
func (t *Tracker) Duration() time.Duration { return time.Since(t.startedAt) }
