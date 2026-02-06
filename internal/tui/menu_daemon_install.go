package tui

import tea "github.com/charmbracelet/bubbletea"

type daemonInstallMenuItem struct{}

func (daemonInstallMenuItem) Title() string {
	return "Install daemon"
}

func (daemonInstallMenuItem) Description() string {
	return "Install launchd daemon"
}

func (daemonInstallMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "Install daemon"
	return *m, tea.Batch(m.runDaemonCmd(actionDaemonInstall), m.spinner.Tick)
}
