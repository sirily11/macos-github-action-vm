package tui

import tea "github.com/charmbracelet/bubbletea"

type quitMenuItem struct{}

func (quitMenuItem) Title() string {
	return "Quit"
}

func (quitMenuItem) Description() string {
	return "Exit"
}

func (quitMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.stopRunnerIfActive()
	m.closeLogFile()
	return *m, tea.Quit
}
