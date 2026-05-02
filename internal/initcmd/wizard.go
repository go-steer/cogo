// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package initcmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/go-steer/cogo/internal/config"
)

// WizardResult is what RunWizard returns when the user confirms.
// Cancelled is true when the user exited via Esc / Ctrl+C; in that
// case Cfg is unset and the caller should write nothing.
type WizardResult struct {
	Cfg       *config.Config
	Cancelled bool
}

// RunWizard launches the Bubble Tea wizard interactively. Returns
// the user's choices wrapped in a WizardResult; the caller decides
// whether to call WriteAgentsDir with them.
func RunWizard() (WizardResult, error) {
	m := newWizardModel()
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return WizardResult{}, fmt.Errorf("wizard: %w", err)
	}
	wm, ok := final.(*wizardModel)
	if !ok {
		return WizardResult{}, errors.New("wizard: unexpected final model type")
	}
	if wm.cancelled || wm.step != stepDone {
		return WizardResult{Cancelled: true}, nil
	}
	cfg := config.DefaultConfig()
	cfg.Model.Provider = wm.provider
	cfg.Model.Name = wm.modelName
	cfg.Permissions.Mode = wm.permMode
	return WizardResult{Cfg: cfg}, nil
}

// step indexes the sequential wizard panels.
type step int

const (
	stepProvider step = iota
	stepModel
	stepPermMode
	stepConfirm
	stepDone
)

type wizardModel struct {
	step      step
	cancelled bool

	// provider step
	providerChoices []string
	providerCursor  int
	provider        string

	// model step
	modelInput textinput.Model
	modelName  string

	// perm mode step
	permChoices []string
	permCursor  int
	permMode    string

	width  int
	height int
}

func newWizardModel() *wizardModel {
	ti := textinput.New()
	ti.Placeholder = "gemini-3.1-pro-preview"
	ti.SetValue("gemini-3.1-pro-preview")
	ti.CharLimit = 80
	ti.Width = 40

	return &wizardModel{
		providerChoices: []string{"gemini", "vertex"},
		permChoices:     []string{"ask", "allow", "yolo"},
		modelInput:      ti,
	}
}

func (m *wizardModel) Init() tea.Cmd { return textinput.Blink }

func (m *wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
		switch m.step {
		case stepProvider:
			return m.updateProvider(msg)
		case stepModel:
			return m.updateModel(msg)
		case stepPermMode:
			return m.updatePerm(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m *wizardModel) updateProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(m.providerChoices)-1 {
			m.providerCursor++
		}
	case "enter":
		m.provider = m.providerChoices[m.providerCursor]
		m.step = stepModel
		m.modelInput.Focus()
	}
	return m, nil
}

func (m *wizardModel) updateModel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := m.modelInput.Value()
		if val == "" {
			return m, nil
		}
		m.modelName = val
		m.step = stepPermMode
		return m, nil
	}
	var cmd tea.Cmd
	m.modelInput, cmd = m.modelInput.Update(msg)
	return m, cmd
}

func (m *wizardModel) updatePerm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.permCursor > 0 {
			m.permCursor--
		}
	case "down", "j":
		if m.permCursor < len(m.permChoices)-1 {
			m.permCursor++
		}
	case "enter":
		m.permMode = m.permChoices[m.permCursor]
		m.step = stepConfirm
	}
	return m, nil
}

func (m *wizardModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y":
		m.step = stepDone
		return m, tea.Quit
	case "n":
		// Restart wizard at provider step.
		m.step = stepProvider
		return m, nil
	}
	return m, nil
}

func (m *wizardModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("cogo init") + "\n\n"
	switch m.step {
	case stepProvider:
		return title + "Provider:\n" + renderChoices(m.providerChoices, m.providerCursor) +
			"\n\n↑/↓ select · enter · esc cancel"
	case stepModel:
		return title + "Model name:\n" + m.modelInput.View() +
			"\n\nenter · esc cancel"
	case stepPermMode:
		return title + "Permission mode:\n" + renderChoices(m.permChoices, m.permCursor) +
			"\n\n↑/↓ select · enter · esc cancel"
	case stepConfirm:
		return title + fmt.Sprintf("Confirm:\n  provider: %s\n  model:    %s\n  mode:     %s\n\nenter / y to write · n to start over · esc cancel",
			m.provider, m.modelName, m.permMode)
	}
	return ""
}

func renderChoices(choices []string, cursor int) string {
	out := ""
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#005f87", Dark: "#5fafff"})
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6c6c6c", Dark: "#9a9a9a"})
	for i, c := range choices {
		marker := "  "
		if i == cursor {
			marker = "▸ "
			out += cursorStyle.Render(marker+c) + "\n"
			continue
		}
		out += mutedStyle.Render(marker+c) + "\n"
	}
	return out
}
