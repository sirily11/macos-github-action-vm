package tui

import tea "github.com/charmbracelet/bubbletea"

type monitorDaemonUninstallMenuItem struct{}

func (monitorDaemonUninstallMenuItem) Title() string {
	return "Uninstall monitor daemon"
}

func (monitorDaemonUninstallMenuItem) Description() string {
	return "Uninstall launchd daemon for log monitoring"
}

func (monitorDaemonUninstallMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Uninstall monitor daemon"
	return *m, tea.Batch(m.runDaemonCmd(actionMonitorDaemonUninstall), m.spinner.Tick)
}
