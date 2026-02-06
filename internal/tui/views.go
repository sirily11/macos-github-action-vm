package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) viewMenu() string {
	status := "Idle"
	if m.busy {
		status = "Busy: " + m.busyLabel
	}
	if m.runnerActive {
		status = status + " | Runner active (press s to stop)"
	}
	if m.busy {
		status = m.spinner.View() + " " + status
	}

	logLine := "Logs: " + m.logPath
	if m.logPath == "" {
		logLine = "Logs: (disabled)"
	}

	latest := m.lastLogLine
	if latest == "" {
		latest = "(no log output yet)"
	}
	latest = fitLine(latest, m.windowWidth)

	lastError := ""
	if m.lastError != "" {
		lastError = "\n\nLast error: " + m.lastError
	}

	header := headerView("RVMM", status, latest)
	tips := "enter=select  s=stop runner  q=quit"
	if len(m.menuStack) > 0 {
		tips = "enter=select  esc=back  s=stop runner  q=quit"
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s%s\n\nTips: %s", header, m.menu.View(), logLine, lastError, tips)
}

func (m model) viewConfig() string {
	var b strings.Builder
	b.WriteString("Edit configuration (esc to cancel)\n\n")
	for i, input := range m.configForm.inputs {
		field := m.configForm.fields[i]
		cursor := " "
		if m.configForm.focusIndex == i {
			cursor = ">"
		}
		required := ""
		if field.required {
			required = "*"
		}
		b.WriteString(fmt.Sprintf("%s %s%s: %s\n", cursor, field.label, required, input.View()))
	}
	if m.configForm.errMsg != "" {
		b.WriteString("\nError: " + m.configForm.errMsg + "\n")
	}
	b.WriteString("\nTab/Up/Down to move, Enter to save")
	return b.String()
}

func (m model) viewBuildPrompt() string {
	return "Build VM image\n\n" + m.buildInput.View() + "\n\nEnter to start, Esc to cancel"
}

func (m model) viewPushPrompt() string {
	return "Push image to GHCR\n\nTarget image name (ghcr.io/owner/image:tag):\n" + m.pushInput.View() + "\n\nEnter to push, Esc to cancel"
}

func (m model) viewPullPrompt() string {
	return "Pull image from registry\n\nImage name (ghcr.io/owner/image:tag):\n" + m.pullInput.View() + "\n\nEnter to pull, Esc to cancel"
}

func (m model) viewLogScreen() string {
	if m.logPath == "" {
		return "Logs are disabled."
	}
	return "Logs are written to:\n\n" + m.logPath + "\n\nOpen the file to view full output."
}

func headerView(title, status, latest string) string {
	badge := lipgloss.NewStyle().
		Padding(0, 1).
		Background(lipgloss.Color("60")).
		Foreground(lipgloss.Color("230")).
		Bold(true).
		Render(title)

	statusStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(lipgloss.Color("229"))

	latestStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(lipgloss.Color("230"))

	firstLine := lipgloss.JoinHorizontal(lipgloss.Left, badge, statusStyle.Render(status))
	secondLine := latestStyle.Render(latest)
	return lipgloss.JoinVertical(lipgloss.Left, firstLine, secondLine)
}
