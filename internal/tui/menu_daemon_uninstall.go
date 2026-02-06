package tui

import tea "github.com/charmbracelet/bubbletea"

type daemonUninstallMenuItem struct{}

func (daemonUninstallMenuItem) Title() string {
	return "Uninstall daemon"
}

func (daemonUninstallMenuItem) Description() string {
	return "Remove launchd daemon"
}

func (daemonUninstallMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Uninstall daemon"
	return *m, tea.Batch(m.runDaemonCmd(actionDaemonUninstall), m.spinner.Tick)
}
