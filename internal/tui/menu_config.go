package tui

import tea "github.com/charmbracelet/bubbletea"

type configMenuItem struct{}

func (configMenuItem) Title() string {
	return "Create/edit config"
}

func (configMenuItem) Description() string {
	return "Edit rvmm.yaml in project root"
}

func (configMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	cfg := loadConfigOrDefault(m.configPath)
	m.configForm = newConfigForm(cfg)
	m.state = stateConfig
	return *m, nil
}
