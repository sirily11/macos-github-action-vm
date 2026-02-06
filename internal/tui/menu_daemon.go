package tui

import tea "github.com/charmbracelet/bubbletea"

type daemonMenuItem struct{}

func (daemonMenuItem) Title() string {
	return "Daemon"
}

func (daemonMenuItem) Description() string {
	return "Install, uninstall, or check daemon status"
}

func (daemonMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	submenu := newMenuModel(daemonMenuEntries())
	m.pushMenu(submenu)
	return *m, nil
}

func daemonMenuEntries() []menuEntry {
	return []menuEntry{
		daemonInstallMenuItem{},
		daemonUninstallMenuItem{},
		daemonStatusMenuItem{},
	}
}
