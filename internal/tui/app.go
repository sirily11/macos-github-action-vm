package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rxtech-lab/rvmm/internal/daemon"
	"github.com/rxtech-lab/rvmm/internal/runner"
	"github.com/rxtech-lab/rvmm/internal/setup"
	"go.uber.org/zap"
)

type appState int

type actionType int

const (
	stateMenu appState = iota
	stateConfig
	stateBuildPrompt
	statePushPrompt
	statePullPrompt
	stateLogs
)

const (
	actionSetup actionType = iota
	actionBuild
	actionConfig
	actionRun
	actionListImages
	actionPushImage
	actionPullImage
	actionDaemonInstall
	actionDaemonUninstall
	actionDaemonStatus
	actionMonitorDaemonInstall
	actionMonitorDaemonUninstall
	actionMonitorDaemonStatus
	actionViewLogs
	actionQuit
)

type taskDoneMsg struct {
	action actionType
	err    error
}

type logTailMsg struct {
	line string
}

type model struct {
	state        appState
	menu         list.Model
	menuStack    []list.Model
	configForm   configForm
	buildInput   textinput.Model
	pushInput    textinput.Model
	pullInput    textinput.Model
	spinner      spinner.Model
	logger       *zap.Logger
	logWriter    io.Writer
	logCloser    io.Closer
	logPath      string
	configPath   string
	busy         bool
	busyLabel    string
	runnerActive bool
	runnerCancel context.CancelFunc
	windowWidth  int
	windowHeight int
	lastError    string
	lastLogLine  string
}

func Run() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("TUI error:", err)
		os.Exit(1)
	}
}

func newModel() model {
	menu := newMenuModel(rootMenuEntries())

	buildInput := textinput.New()
	buildInput.Placeholder = "Press Enter to build runner"
	buildInput.CharLimit = 512
	buildInput.Width = 50

	pushInput := textinput.New()
	pushInput.Placeholder = "ghcr.io/owner/image:tag"
	pushInput.CharLimit = 512
	pushInput.Width = 50

	pullInput := textinput.New()
	pullInput.Placeholder = "ghcr.io/owner/image:tag"
	pullInput.CharLimit = 512
	pullInput.Width = 50

	spin := spinner.New()
	spin.Spinner = spinner.Line

	logger, logWriter, logCloser, logPath, logErr := newLogger()

	configPath := defaultConfigPath()

	m := model{
		state:      stateMenu,
		menu:       menu,
		buildInput: buildInput,
		pushInput:  pushInput,
		pullInput:  pullInput,
		spinner:    spin,
		logger:     logger,
		logWriter:  logWriter,
		logCloser:  logCloser,
		logPath:    logPath,
		configPath: configPath,
	}

	if logErr != nil {
		m.lastError = logErr.Error()
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tickLogTail(m.logPath)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateMenu:
			return m.updateMenu(msg)
		case stateConfig:
			return m.updateConfig(msg)
		case stateBuildPrompt:
			return m.updateBuildPrompt(msg)
		case statePushPrompt:
			return m.updatePushPrompt(msg)
		case statePullPrompt:
			return m.updatePullPrompt(msg)
		case stateLogs:
			return m.updateLogScreen(msg)
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.resizeMenus(msg.Width, msg.Height)
	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case logTailMsg:
		if msg.line != "" {
			m.lastLogLine = msg.line
		}
		return m, tickLogTail(m.logPath)
	case taskDoneMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.action == actionRun {
			m.runnerActive = false
			m.runnerCancel = nil
		}
		if msg.err != nil {
			m.lastError = msg.err.Error()
		} else {
			m.lastError = ""
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case stateMenu:
		if m.busy {
			return m, nil
		}
		m.menu, cmd = m.menu.Update(msg)
	case stateConfig:
		m.configForm, cmd = m.configForm.Update(msg)
	case stateBuildPrompt:
		m.buildInput, cmd = m.buildInput.Update(msg)
	case statePushPrompt:
		m.pushInput, cmd = m.pushInput.Update(msg)
	case statePullPrompt:
		m.pullInput, cmd = m.pullInput.Update(msg)
	case stateLogs:
		return m, nil
	}

	return m, cmd
}

func (m model) View() string {
	switch m.state {
	case stateConfig:
		return m.viewConfig()
	case stateBuildPrompt:
		return m.viewBuildPrompt()
	case statePushPrompt:
		return m.viewPushPrompt()
	case statePullPrompt:
		return m.viewPullPrompt()
	case stateLogs:
		return m.viewLogScreen()
	default:
		return m.viewMenu()
	}
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.stopRunnerIfActive()
		m.closeLogFile()
		return m, tea.Quit
	case "esc":
		if m.popMenu() {
			return m, nil
		}
		return m, nil
	case "enter":
		if m.busy {
			m.lastError = "busy: " + m.busyLabel
			return m, nil
		}
		item, ok := m.menu.SelectedItem().(menuListItem)
		if !ok {
			return m, nil
		}
		return item.entry.OnSelect(&m)
	case "s":
		if m.runnerActive {
			m.lastError = ""
			m.stopRunnerIfActive()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m model) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateMenu
		return m, nil
	case "enter":
		if m.configForm.focusIndex == len(m.configForm.inputs)-1 {
			cfg, err := m.configForm.toConfig()
			if err != nil {
				m.configForm.errMsg = err.Error()
				return m, nil
			}
			if err := writeConfig(m.configPath, cfg); err != nil {
				m.configForm.errMsg = err.Error()
				return m, nil
			}
			m.lastError = ""
			m.state = stateMenu
			return m, nil
		}
	case "tab", "shift+tab", "up", "down":
		m.configForm = m.configForm.updateFocus(msg.String())
		return m, nil
	}

	var cmd tea.Cmd
	m.configForm, cmd = m.configForm.Update(msg)
	return m, cmd
}

func (m model) updateBuildPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateMenu
		return m, nil
	case "enter":
		m.state = stateMenu
		m.busy = true
		m.busyLabel = "Build VM image"
		return m, tea.Batch(m.runBuildCmd(m.buildInput.Value()), m.spinner.Tick)
	}

	var cmd tea.Cmd
	m.buildInput, cmd = m.buildInput.Update(msg)
	return m, cmd
}

func (m model) updatePushPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateMenu
		return m, nil
	case "enter":
		image := strings.TrimSpace(m.pushInput.Value())
		if image == "" {
			m.lastError = "image name is required"
			return m, nil
		}
		m.state = stateMenu
		m.busy = true
		m.busyLabel = "Push image"
		return m, tea.Batch(m.runPushImageCmd(image), m.spinner.Tick)
	}

	var cmd tea.Cmd
	m.pushInput, cmd = m.pushInput.Update(msg)
	return m, cmd
}

func (m model) updatePullPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateMenu
		return m, nil
	case "enter":
		image := strings.TrimSpace(m.pullInput.Value())
		if image == "" {
			m.lastError = "image name is required"
			return m, nil
		}
		m.state = stateMenu
		m.busy = true
		m.busyLabel = "Pull image"
		return m, tea.Batch(m.runPullImageCmd(image), m.spinner.Tick)
	}

	var cmd tea.Cmd
	m.pullInput, cmd = m.pullInput.Update(msg)
	return m, cmd
}

func (m model) updateLogScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = stateMenu
		return m, nil
	}

	return m, nil
}

func (m *model) stopRunnerIfActive() {
	if m.runnerCancel != nil {
		m.runnerCancel()
		m.runnerCancel = nil
	}
	m.runnerActive = false
}

func (m *model) closeLogFile() {
	if m.logCloser != nil {
		_ = m.logCloser.Close()
		m.logCloser = nil
	}
}

func (m model) runSetupCmd() tea.Cmd {
	return func() tea.Msg {
		if err := setup.RunWithIO(m.logger, m.logWriter, m.logWriter, os.Stdin); err != nil {
			return taskDoneMsg{action: actionSetup, err: err}
		}
		return taskDoneMsg{action: actionSetup, err: nil}
	}
}

func (m model) runBuildCmd(ipsw string) tea.Cmd {
	return func() tea.Msg {
		guestDir := "guest"
		if err := runCommandSeries(m.logWriter, guestDir, buildCommands(ipsw)...); err != nil {
			return taskDoneMsg{action: actionBuild, err: err}
		}
		return taskDoneMsg{action: actionBuild, err: nil}
	}
}

func (m model) runRunnerCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		cfg, err := loadConfig(m.configPath)
		if err != nil {
			return taskDoneMsg{action: actionRun, err: err}
		}
		if err := cfg.Validate(); err != nil {
			return taskDoneMsg{action: actionRun, err: err}
		}
		err = runner.Run(ctx, m.logger, cfg)
		return taskDoneMsg{action: actionRun, err: err}
	}
}

func (m model) runPushImageCmd(target string) tea.Cmd {
	return func() tea.Msg {
		if err := runCommandStreaming(m.logWriter, exec.Command("tart", "push", "runner", target)); err != nil {
			return taskDoneMsg{action: actionPushImage, err: err}
		}
		return taskDoneMsg{action: actionPushImage, err: nil}
	}
}

func (m model) runPullImageCmd(target string) tea.Cmd {
	return func() tea.Msg {
		if err := runCommandStreaming(m.logWriter, exec.Command("tart", "pull", target)); err != nil {
			return taskDoneMsg{action: actionPullImage, err: err}
		}
		return taskDoneMsg{action: actionPullImage, err: nil}
	}
}

func (m model) runListImagesCmd() tea.Cmd {
	return func() tea.Msg {
		if err := runCommandStreaming(m.logWriter, exec.Command("tart", "list")); err != nil {
			return taskDoneMsg{action: actionListImages, err: err}
		}

		paths, err := listTartVMPaths()
		if err != nil {
			return taskDoneMsg{action: actionListImages, err: err}
		}
		if len(paths) == 0 {
			_, _ = fmt.Fprintln(m.logWriter, "No local Tart images found.")
			return taskDoneMsg{action: actionListImages, err: nil}
		}

		args := append([]string{"-sh"}, paths...)
		if err := runCommandStreaming(m.logWriter, exec.Command("du", args...)); err != nil {
			return taskDoneMsg{action: actionListImages, err: err}
		}
		return taskDoneMsg{action: actionListImages, err: nil}
	}
}

func (m model) runDaemonCmd(action actionType) tea.Cmd {
	return func() tea.Msg {
		cfg, err := loadConfig(m.configPath)
		if err != nil {
			return taskDoneMsg{action: action, err: err}
		}

		switch action {
		case actionDaemonInstall:
			if err := cfg.Validate(); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
			if err := daemon.Install(m.logger, cfg, m.configPath, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
			// Verify daemon is running after install
			running, err := daemon.IsRunning(cfg)
			if err != nil {
				m.logger.Warn("Failed to verify daemon status", zap.Error(err))
			} else if !running {
				fmt.Fprintln(m.logWriter, "\n⚠️  Daemon installed but not running. Check logs or try reinstalling.")
			}
		case actionDaemonUninstall:
			if err := daemon.Uninstall(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		case actionDaemonStatus:
			if err := daemon.Status(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		case actionMonitorDaemonInstall:
			if !cfg.PostHog.Enabled {
				return taskDoneMsg{action: action, err: errors.New("PostHog must be enabled in config")}
			}
			if err := cfg.Validate(); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
			if err := daemon.InstallMonitor(m.logger, cfg, m.configPath, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		case actionMonitorDaemonUninstall:
			if err := daemon.UninstallMonitor(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		case actionMonitorDaemonStatus:
			if err := daemon.StatusMonitor(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		default:
			return taskDoneMsg{action: action, err: errors.New("unsupported daemon action")}
		}

		return taskDoneMsg{action: action, err: nil}
	}
}
