package tui

import tea "github.com/charmbracelet/bubbletea"

type listImagesMenuItem struct{}

func (listImagesMenuItem) Title() string {
	return "List images"
}

func (listImagesMenuItem) Description() string {
	return "Show Tart images and sizes"
}

func (listImagesMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	m.busy = true
	m.busyLabel = "List images"
	return *m, tea.Batch(m.runListImagesCmd(), m.spinner.Tick)
}
