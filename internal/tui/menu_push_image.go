package tui

import tea "github.com/charmbracelet/bubbletea"

type pushImageMenuItem struct{}

func (pushImageMenuItem) Title() string {
	return "Push image"
}

func (pushImageMenuItem) Description() string {
	return "Push local image to GHCR"
}

func (pushImageMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.state = statePushPrompt
	m.pushInput.SetValue("")
	m.pushInput.Focus()
	return *m, nil
}
