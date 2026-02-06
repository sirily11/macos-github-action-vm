package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type runMenuItem struct{}

func (runMenuItem) Title() string {
	return "Run runner"
}

func (runMenuItem) Description() string {
	return "Start the runner loop"
}

func (runMenuItem) OnSelect(m *model) (tea.Model, tea.Cmd) {
	if m.runnerActive {
		m.lastError = "runner already active"
		return *m, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.busy = true
	m.busyLabel = "Runner loop"
	m.runnerActive = true
	m.runnerCancel = cancel
	return *m, tea.Batch(m.runRunnerCmd(ctx), m.spinner.Tick)
}
