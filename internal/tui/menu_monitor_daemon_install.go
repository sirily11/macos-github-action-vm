package tui

import tea "github.com/charmbracelet/bubbletea"

type monitorDaemonInstallMenuItem struct{}

func (monitorDaemonInstallMenuItem) Title() string {
	return "Install monitor daemon"
}

func (monitorDaemonInstallMenuItem) Description() string {
	return "Install launchd daemon for log monitoring"
}

func (monitorDaemonInstallMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Install monitor daemon"
	return *m, tea.Batch(m.runDaemonCmd(actionMonitorDaemonInstall), m.spinner.Tick)
}
