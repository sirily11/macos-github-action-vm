package tui

import tea "github.com/charmbracelet/bubbletea"

type monitorDaemonMenuItem struct{}

func (monitorDaemonMenuItem) Title() string {
	return "Monitor Daemon"
}

func (monitorDaemonMenuItem) Description() string {
	return "Install, uninstall, or check monitor daemon status"
}

func (monitorDaemonMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	submenu := newMenuModel(monitorDaemonMenuEntries())
	m.pushMenu(submenu)
	return *m, nil
}

func monitorDaemonMenuEntries() []menuEntry {
	return []menuEntry{
		monitorDaemonInstallMenuItem{},
		monitorDaemonUninstallMenuItem{},
		monitorDaemonStatusMenuItem{},
	}
}
