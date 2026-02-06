package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type menuEntry interface {
	Title() string
	Description() string
	OnSelect(*model) (tea.Model, tea.Cmd)
}

type menuListItem struct {
	entry menuEntry
}

func (m menuListItem) Title() string {
	return m.entry.Title()
}

func (m menuListItem) Description() string {
	return m.entry.Description()
}

func (m menuListItem) FilterValue() string {
	return m.entry.Title()
}

func newMenuModel(entries []menuEntry) list.Model {
	items := make([]list.Item, len(entries))
	for i, entry := range entries {
		items[i] = menuListItem{entry: entry}
	}

	menu := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menu.Title = ""
	menu.SetShowTitle(false)
	menu.SetShowStatusBar(false)
	menu.SetShowHelp(false)
	menu.SetFilteringEnabled(false)
	return menu
}

func menuHeight(windowHeight int) int {
	return max(4, windowHeight-14)
}

func (m *model) pushMenu(menu list.Model) {
	menu.SetSize(m.windowWidth, menuHeight(m.windowHeight))
	m.menuStack = append(m.menuStack, m.menu)
	m.menu = menu
}

func (m *model) popMenu() bool {
	if len(m.menuStack) == 0 {
		return false
	}
	last := m.menuStack[len(m.menuStack)-1]
	m.menuStack = m.menuStack[:len(m.menuStack)-1]
	m.menu = last
	return true
}

func (m *model) resizeMenus(width, height int) {
	size := menuHeight(height)
	m.menu.SetSize(width, size)
	for i := range m.menuStack {
		m.menuStack[i].SetSize(width, size)
	}
}
