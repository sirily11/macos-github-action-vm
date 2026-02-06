package tui

import tea "github.com/charmbracelet/bubbletea"

type pullImageMenuItem struct{}

func (pullImageMenuItem) Title() string {
	return "Pull image"
}

func (pullImageMenuItem) Description() string {
	return "Pull image from registry"
}

func (pullImageMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.state = statePullPrompt
	m.pullInput.SetValue("")
	m.pullInput.Focus()
	return *m, nil
}
