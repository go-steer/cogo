package initcmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg is a tiny helper for building tea.KeyMsg values from a
// string representation (matches msg.String() for navigation keys).
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// drive feeds keys into the model and returns the final wizardModel.
// Stops as soon as Quit is signaled (cmd != nil and equal to tea.Quit's
// underlying type) — for the wizard, Quit fires on cancel or confirm.
func drive(t *testing.T, m *wizardModel, keys ...string) *wizardModel {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(keyMsg(k))
		m = next.(*wizardModel)
	}
	return m
}

func TestWizard_FullHappyPath(t *testing.T) {
	t.Parallel()
	m := newWizardModel()

	// Step 1: provider — default cursor at gemini, Down to vertex, Enter.
	m = drive(t, m, "down", "enter")
	if m.provider != "vertex" {
		t.Fatalf("provider = %q, want vertex", m.provider)
	}
	if m.step != stepModel {
		t.Fatalf("step after provider = %v, want stepModel", m.step)
	}

	// Step 2: model — default text is "gemini-3.1-pro-preview"; just Enter.
	m = drive(t, m, "enter")
	if m.modelName != "gemini-3.1-pro-preview" {
		t.Fatalf("modelName = %q", m.modelName)
	}
	if m.step != stepPermMode {
		t.Fatalf("step after model = %v, want stepPermMode", m.step)
	}

	// Step 3: permission mode — default cursor at "ask", Down twice = "yolo".
	m = drive(t, m, "down", "down", "enter")
	if m.permMode != "yolo" {
		t.Fatalf("permMode = %q, want yolo", m.permMode)
	}
	if m.step != stepConfirm {
		t.Fatalf("step after perm = %v, want stepConfirm", m.step)
	}

	// Confirm — Enter.
	m = drive(t, m, "enter")
	if m.step != stepDone {
		t.Fatalf("step after confirm = %v, want stepDone", m.step)
	}
	if m.cancelled {
		t.Errorf("cancelled should be false on confirm path")
	}
}

func TestWizard_EscCancels(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	m = drive(t, m, "esc")
	if !m.cancelled {
		t.Errorf("expected cancelled=true after esc")
	}
}

func TestWizard_CtrlCCancels(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	m = drive(t, m, "ctrl+c")
	if !m.cancelled {
		t.Errorf("expected cancelled=true after ctrl+c")
	}
}

func TestWizard_ConfirmNStartsOver(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	// Walk to confirm step.
	m = drive(t, m, "enter") // pick gemini
	m = drive(t, m, "enter") // accept default model
	m = drive(t, m, "enter") // pick ask mode
	if m.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %v", m.step)
	}
	// Press n to restart.
	m = drive(t, m, "n")
	if m.step != stepProvider {
		t.Errorf("expected stepProvider after n, got %v", m.step)
	}
}

func TestWizard_ProviderUpDownClamps(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	// Up from cursor=0 should stay at 0.
	m = drive(t, m, "up", "up")
	if m.providerCursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", m.providerCursor)
	}
	// Down past end clamps too.
	m = drive(t, m, "down", "down", "down")
	if m.providerCursor != len(m.providerChoices)-1 {
		t.Errorf("cursor = %d, want last index (clamped)", m.providerCursor)
	}
}

func TestWizard_ModelInputAcceptsTyping(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	m = drive(t, m, "enter") // advance to model step
	// Clear default and type a custom name.
	for i := 0; i < 30; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = next.(*wizardModel)
	}
	for _, r := range "custom-model" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(*wizardModel)
	}
	if v := m.modelInput.Value(); v != "custom-model" {
		t.Errorf("textinput value = %q, want custom-model", v)
	}
}

func TestWizard_ViewRendersEachStep(t *testing.T) {
	t.Parallel()
	m := newWizardModel()
	for _, want := range []struct {
		step  step
		label string
	}{
		{stepProvider, "Provider:"},
		{stepModel, "Model name:"},
		{stepPermMode, "Permission mode:"},
		{stepConfirm, "Confirm:"},
	} {
		m.step = want.step
		// stepConfirm reads the captured selections; pre-populate sane
		// defaults so the format string doesn't render zero values.
		if want.step == stepConfirm {
			m.provider, m.modelName, m.permMode = "gemini", "x", "ask"
		}
		if !strings.Contains(m.View(), want.label) {
			t.Errorf("step %v missing label %q", want.step, want.label)
		}
	}
}
