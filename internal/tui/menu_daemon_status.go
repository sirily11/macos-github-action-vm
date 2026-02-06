package tui

import tea "github.com/charmbracelet/bubbletea"

type daemonStatusMenuItem struct{}

func (daemonStatusMenuItem) Title() string {
	return "Daemon status"
}

func (daemonStatusMenuItem) Description() string {
	return "Show launchd status"
}

func (daemonStatusMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Daemon status"
	return *m, tea.Batch(m.runDaemonCmd(actionDaemonStatus), m.spinner.Tick)
}
