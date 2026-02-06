package tui

import tea "github.com/charmbracelet/bubbletea"

type setupMenuItem struct{}

func (setupMenuItem) Title() string {
	return "Setup dependencies"
}

func (setupMenuItem) Description() string {
	return "Install brew packages and create config"
}

func (setupMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Setup dependencies"
	return *m, tea.Batch(m.runSetupCmd(), m.spinner.Tick)
}
