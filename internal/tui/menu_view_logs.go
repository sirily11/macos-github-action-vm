package tui

import tea "github.com/charmbracelet/bubbletea"

type viewLogsMenuItem struct{}

func (viewLogsMenuItem) Title() string {
	return "View logs"
}

func (viewLogsMenuItem) Description() string {
	return "Open log viewer"
}

func (viewLogsMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.state = stateLogs
	return *m, nil
}
