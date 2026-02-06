package tui

import tea "github.com/charmbracelet/bubbletea"

type buildMenuItem struct{}

func (buildMenuItem) Title() string {
	return "Build VM image"
}

func (buildMenuItem) Description() string {
	return "Run Packer/Tart build for runner"
}

func (buildMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.state = stateBuildPrompt
	m.buildInput.SetValue("")
	m.buildInput.Focus()
	return *m, nil
}
