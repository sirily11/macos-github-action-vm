package monitor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rxtech-lab/rvmm/internal/posthog"
	"go.uber.org/zap"
)

// LogTailer monitors a log file and sends new lines to PostHog
type LogTailer struct {
	filePath   string
	logType    string
	posthog    *posthog.Client
	log        *zap.Logger
	offset     int64
	pollPeriod time.Duration
}

// NewLogTailer creates a new log tailer
func NewLogTailer(filePath string, logType string, posthog *posthog.Client, log *zap.Logger) *LogTailer {
	return &LogTailer{
		filePath:   filePath,
		logType:    logType,
		posthog:    posthog,
		log:        log,
		offset:     0,
		pollPeriod: 2 * time.Second,
	}
}

// Start begins monitoring the log file
func (t *LogTailer) Start(ctx context.Context) error {
	t.log.Info("Starting log tailer",
		zap.String("file", t.filePath),
		zap.String("log_type", t.logType),
	)

	// Try to seek to end of existing file if it exists
	if err := t.seekToEnd(); err != nil {
		t.log.Warn("Could not seek to end of file, starting from beginning",
			zap.String("file", t.filePath),
			zap.Error(err),
		)
	}

	ticker := time.NewTicker(t.pollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.log.Info("Log tailer stopped",
				zap.String("file", t.filePath),
			)
			return ctx.Err()
		case <-ticker.C:
			if err := t.checkAndReadNewLines(); err != nil {
				t.log.Error("Error reading log file",
					zap.String("file", t.filePath),
					zap.Error(err),
				)
			}
		}
	}
}

// seekToEnd moves the offset to the end of the file if it exists
func (t *LogTailer) seekToEnd() error {
	file, err := os.Open(t.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's okay
		}
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	t.offset = stat.Size()
	t.log.Info("Positioned at end of file",
		zap.String("file", t.filePath),
		zap.Int64("offset", t.offset),
	)
	return nil
}

// checkAndReadNewLines checks the file for new content and reads it
func (t *LogTailer) checkAndReadNewLines() error {
	file, err := os.Open(t.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, just wait
			return nil
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	currentSize := stat.Size()

	// Check if file was truncated or rotated
	if currentSize < t.offset {
		t.log.Info("File truncated or rotated, resetting to beginning",
			zap.String("file", t.filePath),
			zap.Int64("old_offset", t.offset),
			zap.Int64("new_size", currentSize),
		)
		t.offset = 0
	}

	// Check if there's new content
	if currentSize == t.offset {
		// No new content
		return nil
	}

	// Seek to our last position
	if _, err := file.Seek(t.offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Read new lines
	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Send to PostHog
		if err := t.posthog.CaptureLogEvent(t.logType, line); err != nil {
			t.log.Error("Failed to send log to PostHog",
				zap.String("log_type", t.logType),
				zap.Error(err),
			)
			// Continue processing other lines even if one fails
		}
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	// Update offset to current position
	newOffset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current offset: %w", err)
	}
	t.offset = newOffset

	if lineCount > 0 {
		t.log.Info("Processed new log lines",
			zap.String("file", t.filePath),
			zap.Int("count", lineCount),
			zap.Int64("new_offset", t.offset),
		)
	}

	return nil
}
