package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

func defaultLogPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return ".rvmm.log"
	}
	return filepath.Join(workingDir, ".rvmm.log")
}
