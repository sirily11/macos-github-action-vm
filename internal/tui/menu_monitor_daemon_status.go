package tui

import tea "github.com/charmbracelet/bubbletea"

type monitorDaemonStatusMenuItem struct{}

func (monitorDaemonStatusMenuItem) Title() string {
	return "Check monitor daemon status"
}

func (monitorDaemonStatusMenuItem) Description() string {
	return "Check if monitor daemon is running"
}

func (monitorDaemonStatusMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Check monitor daemon status"
	return *m, tea.Batch(m.runDaemonCmd(actionMonitorDaemonStatus), m.spinner.Tick)
}
