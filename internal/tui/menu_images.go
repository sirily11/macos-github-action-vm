package tui

import tea "github.com/charmbracelet/bubbletea"

type imagesMenuItem struct{}

func (imagesMenuItem) Title() string {
	return "Images"
}

func (imagesMenuItem) Description() string {
	return "List, push, or pull Tart images"
}

func (imagesMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	submenu := newMenuModel(imagesMenuEntries())
	m.pushMenu(submenu)
	return *m, nil
}

func imagesMenuEntries() []menuEntry {
	return []menuEntry{
		listImagesMenuItem{},
		pushImageMenuItem{},
		pullImageMenuItem{},
	}
}
