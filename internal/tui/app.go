package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/daemon"
	"github.com/rxtech-lab/rvmm/internal/runner"
	"github.com/rxtech-lab/rvmm/internal/setup"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
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
	actionViewLogs
	actionQuit
)

type menuItem struct {
	title       string
	description string
	action      actionType
}

func (m menuItem) Title() string {
	return m.title
}

func (m menuItem) Description() string {
	return m.description
}

func (m menuItem) FilterValue() string {
	return m.title
}

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

type configField struct {
	key      string
	label    string
	required bool
	secret   bool
}

type configForm struct {
	fields     []configField
	inputs     []textinput.Model
	focusIndex int
	errMsg     string
}

func Run() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("TUI error:", err)
		os.Exit(1)
	}
}

func newModel() model {
	items := []list.Item{
		menuItem{title: "Setup dependencies", description: "Install brew packages and create config", action: actionSetup},
		menuItem{title: "Build VM image", description: "Run Packer/Tart build for base and runner", action: actionBuild},
		menuItem{title: "Create/edit config", description: "Edit rvmm.yaml in project root", action: actionConfig},
		menuItem{title: "Run runner", description: "Start the runner loop", action: actionRun},
		menuItem{title: "List images", description: "Show Tart images and sizes", action: actionListImages},
		menuItem{title: "Push image", description: "Push local image to GHCR", action: actionPushImage},
		menuItem{title: "Pull image", description: "Pull image from registry", action: actionPullImage},
		menuItem{title: "Install daemon", description: "Install launchd daemon", action: actionDaemonInstall},
		menuItem{title: "Uninstall daemon", description: "Remove launchd daemon", action: actionDaemonUninstall},
		menuItem{title: "Daemon status", description: "Show launchd status", action: actionDaemonStatus},
		menuItem{title: "View logs", description: "Open log viewer", action: actionViewLogs},
		menuItem{title: "Quit", description: "Exit", action: actionQuit},
	}

	menu := list.New(items, list.NewDefaultDelegate(), 0, 0)
	menu.Title = ""
	menu.SetShowTitle(false)
	menu.SetShowStatusBar(false)
	menu.SetShowHelp(false)
	menu.SetFilteringEnabled(false)

	buildInput := textinput.New()
	buildInput.Placeholder = "Optional IPSW path or URL"
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
		m.menu.SetSize(msg.Width, max(4, msg.Height-14))
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
	case "enter":
		if m.busy {
			m.lastError = "busy: " + m.busyLabel
			return m, nil
		}
		item, ok := m.menu.SelectedItem().(menuItem)
		if !ok {
			return m, nil
		}
		switch item.action {
		case actionQuit:
			m.stopRunnerIfActive()
			m.closeLogFile()
			return m, tea.Quit
		case actionSetup:
			m.busy = true
			m.busyLabel = "Setup dependencies"
			return m, tea.Batch(m.runSetupCmd(), m.spinner.Tick)
		case actionBuild:
			m.state = stateBuildPrompt
			m.buildInput.SetValue("")
			m.buildInput.Focus()
			return m, nil
		case actionConfig:
			cfg := loadConfigOrDefault(m.configPath)
			m.configForm = newConfigForm(cfg)
			m.state = stateConfig
			return m, nil
		case actionRun:
			if m.runnerActive {
				m.lastError = "runner already active"
				return m, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.busy = true
			m.busyLabel = "Runner loop"
			m.runnerActive = true
			m.runnerCancel = cancel
			return m, tea.Batch(m.runRunnerCmd(ctx), m.spinner.Tick)
		case actionListImages:
			m.busy = true
			m.busyLabel = "List images"
			return m, tea.Batch(m.runListImagesCmd(), m.spinner.Tick)
		case actionPushImage:
			m.state = statePushPrompt
			m.pushInput.SetValue("")
			m.pushInput.Focus()
			return m, nil
		case actionPullImage:
			m.state = statePullPrompt
			m.pullInput.SetValue("")
			m.pullInput.Focus()
			return m, nil
		case actionDaemonInstall:
			m.busy = true
			m.busyLabel = "Install daemon"
			return m, tea.Batch(m.runDaemonCmd(actionDaemonInstall), m.spinner.Tick)
		case actionDaemonUninstall:
			m.busy = true
			m.busyLabel = "Uninstall daemon"
			return m, tea.Batch(m.runDaemonCmd(actionDaemonUninstall), m.spinner.Tick)
		case actionDaemonStatus:
			m.busy = true
			m.busyLabel = "Daemon status"
			return m, tea.Batch(m.runDaemonCmd(actionDaemonStatus), m.spinner.Tick)
		case actionViewLogs:
			m.state = stateLogs
			return m, nil
		}
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
	return fmt.Sprintf("%s\n\n%s\n\n%s%s\n\nTips: enter=select  s=stop runner  q=quit", header, m.menu.View(), logLine, lastError)
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
	return "Build VM image\n\nOptional IPSW path or URL (leave empty for default):\n" + m.buildInput.View() + "\n\nEnter to start, Esc to cancel"
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
		case actionDaemonUninstall:
			if err := daemon.Uninstall(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		case actionDaemonStatus:
			if err := daemon.Status(m.logger, cfg, m.logWriter); err != nil {
				return taskDoneMsg{action: action, err: err}
			}
		default:
			return taskDoneMsg{action: action, err: errors.New("unsupported daemon action")}
		}

		return taskDoneMsg{action: action, err: nil}
	}
}

func tickLogTail(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		line := readLastLogLine(path)
		return logTailMsg{line: line}
	})
}

func readLastLogLine(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return ""
	}
	if info.Size() == 0 {
		return ""
	}

	const maxRead = int64(8192)
	readSize := info.Size()
	if readSize > maxRead {
		readSize = maxRead
	}

	start := info.Size() - readSize
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return ""
	}

	buf := make([]byte, readSize)
	if _, err := file.Read(buf); err != nil {
		return ""
	}

	content := strings.TrimRight(string(buf), "\n")
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := sanitizeLogLine(lines[i])
		if line != "" {
			return line
		}
	}

	return ""
}

func listTartVMPaths() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	glob := filepath.Join(homeDir, ".tart", "vms", "*")
	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func sanitizeLogLine(line string) string {
	if line == "" {
		return ""
	}
	line = strings.ReplaceAll(line, "\r", "")
	line = stripANSICodes(line)
	line = strings.TrimSpace(line)
	return line
}

func stripANSICodes(input string) string {
	var b strings.Builder
	b.Grow(len(input))

	state := 0
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch state {
		case 0:
			if ch == 0x1b {
				state = 1
				continue
			}
			if ch < 0x20 && ch != '\t' {
				continue
			}
			b.WriteByte(ch)
		case 1:
			if ch == '[' {
				state = 2
				continue
			}
			state = 0
		case 2:
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				state = 0
			}
		}
	}

	return b.String()
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	if len(line) <= width {
		return line
	}
	if width <= 3 {
		return line[:width]
	}
	return line[:width-3] + "..."
}

func newLogger() (*zap.Logger, io.Writer, io.Closer, string, error) {
	logPath := defaultLogPath()
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return zap.NewNop(), io.Discard, nil, "", fmt.Errorf("open log file: %w", err)
	}

	writer := &safeWriter{w: file}
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.AddSync(writer), zap.InfoLevel)
	return zap.New(core), writer, file, logPath, nil
}

type safeWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *safeWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func runCommandSeries(writer io.Writer, dir string, cmds ...*exec.Cmd) error {
	for _, cmd := range cmds {
		cmd.Dir = dir
		if err := runCommandStreaming(writer, cmd); err != nil {
			return err
		}
	}
	return nil
}

func runCommandStreaming(writer io.Writer, cmd *exec.Cmd) error {
	_, _ = fmt.Fprintf(writer, "$ %s %s\n", cmd.Path, strings.Join(cmd.Args[1:], " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamReader(writer, stdout, &wg)
	go streamReader(writer, stderr, &wg)
	wg.Wait()

	return cmd.Wait()
}

func streamReader(writer io.Writer, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		_, _ = fmt.Fprintln(writer, scanner.Text())
	}
}

func buildCommands(ipsw string) []*exec.Cmd {
	cmds := []*exec.Cmd{
		exec.Command("packer", "init", "base.pkr.hcl"),
	}

	if ipsw != "" {
		cmds = append(cmds, exec.Command("packer", "build", "base.pkr.hcl", "-var", "ipsw="+ipsw))
	} else {
		cmds = append(cmds, exec.Command("packer", "build", "base.pkr.hcl"))
	}

	cmds = append(cmds, exec.Command("packer", "build", "runner.pkr.hcl"))
	return cmds
}

func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s", path)
		}
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadConfigOrDefault(path string) *config.Config {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg
	}

	return defaultConfig()
}

func defaultConfig() *config.Config {
	return &config.Config{
		GitHub: config.GitHubConfig{
			RunnerName:   "runner",
			RunnerLabels: []string{"self-hosted", "arm64"},
		},
		VM: config.VMConfig{
			Username: "admin",
			Password: "admin",
		},
		Options: config.OptionsConfig{
			LogFile:          "runner.log",
			ShutdownFlagFile: ".shutdown",
			WorkingDirectory: "/Users/admin/vm",
		},
		Daemon: config.DaemonConfig{
			Label:     "com.mirego.ekiden",
			PlistPath: "/Library/LaunchDaemons/com.mirego.ekiden.plist",
			User:      "admin",
		},
	}
}

func defaultConfigPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "rvmm.yaml"
	}
	return filepath.Join(workingDir, "rvmm.yaml")
}

func defaultLogPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return ".rvmm.log"
	}
	return filepath.Join(workingDir, ".rvmm.log")
}

func newConfigForm(cfg *config.Config) configForm {
	fields := []configField{
		{key: "github.api_token", label: "GitHub API token", required: true, secret: true},
		{key: "github.registration_endpoint", label: "Registration endpoint", required: true},
		{key: "github.runner_url", label: "Runner URL", required: true},
		{key: "github.runner_name", label: "Runner name"},
		{key: "github.runner_labels", label: "Runner labels (comma)"},
		{key: "vm.username", label: "VM username", required: true},
		{key: "vm.password", label: "VM password", required: true, secret: true},
		{key: "registry.url", label: "Registry URL"},
		{key: "registry.image_name", label: "Registry image name", required: true},
		{key: "registry.username", label: "Registry username"},
		{key: "registry.password", label: "Registry password", secret: true},
		{key: "options.log_file", label: "Log file"},
		{key: "options.shutdown_flag_file", label: "Shutdown flag file"},
		{key: "options.working_directory", label: "Working directory"},
		{key: "daemon.label", label: "Daemon label"},
		{key: "daemon.plist_path", label: "Daemon plist path"},
		{key: "daemon.user", label: "Daemon user"},
	}

	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		input := textinput.New()
		input.CharLimit = 512
		input.Width = 50
		input.SetValue(getFieldValue(cfg, field.key))
		if field.secret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		inputs[i] = input
	}

	if len(inputs) > 0 {
		inputs[0].Focus()
	}

	return configForm{fields: fields, inputs: inputs, focusIndex: 0}
}

func (f configForm) Update(msg tea.Msg) (configForm, tea.Cmd) {
	var cmd tea.Cmd
	for i := range f.inputs {
		if i == f.focusIndex {
			f.inputs[i], cmd = f.inputs[i].Update(msg)
			return f, cmd
		}
	}
	return f, nil
}

func (f configForm) updateFocus(key string) configForm {
	f.inputs[f.focusIndex].Blur()
	switch key {
	case "tab", "down":
		f.focusIndex++
		if f.focusIndex >= len(f.inputs) {
			f.focusIndex = 0
		}
	case "shift+tab", "up":
		f.focusIndex--
		if f.focusIndex < 0 {
			f.focusIndex = len(f.inputs) - 1
		}
	}
	f.inputs[f.focusIndex].Focus()
	return f
}

func (f configForm) toConfig() (*config.Config, error) {
	cfg := defaultConfig()
	for i, field := range f.fields {
		value := strings.TrimSpace(f.inputs[i].Value())
		if field.required && value == "" {
			return nil, fmt.Errorf("%s is required", field.label)
		}
		setFieldValue(cfg, field.key, value)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getFieldValue(cfg *config.Config, key string) string {
	switch key {
	case "github.api_token":
		return cfg.GitHub.APIToken
	case "github.registration_endpoint":
		return cfg.GitHub.RegistrationEndpoint
	case "github.runner_url":
		return cfg.GitHub.RunnerURL
	case "github.runner_name":
		return cfg.GitHub.RunnerName
	case "github.runner_labels":
		return strings.Join(cfg.GitHub.RunnerLabels, ",")
	case "vm.username":
		return cfg.VM.Username
	case "vm.password":
		return cfg.VM.Password
	case "registry.url":
		return cfg.Registry.URL
	case "registry.image_name":
		return cfg.Registry.ImageName
	case "registry.username":
		return cfg.Registry.Username
	case "registry.password":
		return cfg.Registry.Password
	case "options.log_file":
		return cfg.Options.LogFile
	case "options.shutdown_flag_file":
		return cfg.Options.ShutdownFlagFile
	case "options.working_directory":
		return cfg.Options.WorkingDirectory
	case "daemon.label":
		return cfg.Daemon.Label
	case "daemon.plist_path":
		return cfg.Daemon.PlistPath
	case "daemon.user":
		return cfg.Daemon.User
	default:
		return ""
	}
}

func setFieldValue(cfg *config.Config, key, value string) {
	switch key {
	case "github.api_token":
		cfg.GitHub.APIToken = value
	case "github.registration_endpoint":
		cfg.GitHub.RegistrationEndpoint = value
	case "github.runner_url":
		cfg.GitHub.RunnerURL = value
	case "github.runner_name":
		if value != "" {
			cfg.GitHub.RunnerName = value
		}
	case "github.runner_labels":
		if value != "" {
			cfg.GitHub.RunnerLabels = splitCSV(value)
		}
	case "vm.username":
		cfg.VM.Username = value
	case "vm.password":
		cfg.VM.Password = value
	case "registry.url":
		cfg.Registry.URL = value
	case "registry.image_name":
		cfg.Registry.ImageName = value
	case "registry.username":
		cfg.Registry.Username = value
	case "registry.password":
		cfg.Registry.Password = value
	case "options.log_file":
		if value != "" {
			cfg.Options.LogFile = value
		}
	case "options.shutdown_flag_file":
		if value != "" {
			cfg.Options.ShutdownFlagFile = value
		}
	case "options.working_directory":
		if value != "" {
			cfg.Options.WorkingDirectory = value
		}
	case "daemon.label":
		if value != "" {
			cfg.Daemon.Label = value
		}
	case "daemon.plist_path":
		if value != "" {
			cfg.Daemon.PlistPath = value
		}
	case "daemon.user":
		if value != "" {
			cfg.Daemon.User = value
		}
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return labels
}

func writeConfig(path string, cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}
