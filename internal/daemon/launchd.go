package daemon

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/rxtech-lab/rvmm/assets"
	"github.com/rxtech-lab/rvmm/internal/config"
	"go.uber.org/zap"
)

// PlistData contains data for the LaunchDaemon plist template
type PlistData struct {
	Label            string
	BinaryPath       string
	ConfigPath       string
	User             string
	WorkingDirectory string
}

// Install creates and loads the LaunchDaemon
func Install(log *zap.Logger, cfg *config.Config, configPath string, out io.Writer) error {
	log.Info("Installing LaunchDaemon", zap.String("label", cfg.Daemon.Label))

	// Get absolute paths
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute config path: %w", err)
	}

	// Prepare template data
	data := PlistData{
		Label:            cfg.Daemon.Label,
		BinaryPath:       binaryPath,
		ConfigPath:       absConfigPath,
		User:             cfg.Daemon.User,
		WorkingDirectory: cfg.Options.WorkingDirectory,
	}

	// Parse and execute template
	tmpl, err := template.New("plist").Parse(string(assets.EkidenPlist))
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute plist template: %w", err)
	}

	// Ensure working directory exists
	if err := os.MkdirAll(cfg.Options.WorkingDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Write plist file
	plistPath := cfg.Daemon.PlistPath
	if err := os.WriteFile(plistPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write plist (try with sudo): %w", err)
	}

	log.Info("Plist written", zap.String("path", plistPath))

	// Load the daemon
	cmd := exec.Command("launchctl", "load", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load daemon: %w\nOutput: %s", err, string(output))
	}

	log.Info("LaunchDaemon installed and loaded successfully",
		zap.String("label", cfg.Daemon.Label),
		zap.String("plist", plistPath),
	)

	fmt.Fprintf(out, "LaunchDaemon installed: %s\n", cfg.Daemon.Label)
	fmt.Fprintf(out, "Plist location: %s\n", plistPath)
	fmt.Fprintln(out, "\nThe runner will start automatically on boot.")

	return nil
}

// Uninstall unloads and removes the LaunchDaemon
func Uninstall(log *zap.Logger, cfg *config.Config, out io.Writer) error {
	log.Info("Uninstalling LaunchDaemon", zap.String("label", cfg.Daemon.Label))

	plistPath := cfg.Daemon.PlistPath

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		log.Warn("Plist file not found", zap.String("path", plistPath))
		fmt.Fprintln(out, "LaunchDaemon is not installed")
		return nil
	}

	// Unload the daemon
	cmd := exec.Command("launchctl", "unload", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Warn("Failed to unload daemon (may already be unloaded)",
			zap.Error(err),
			zap.String("output", string(output)),
		)
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist (try with sudo): %w", err)
	}

	log.Info("LaunchDaemon uninstalled", zap.String("label", cfg.Daemon.Label))
	fmt.Fprintf(out, "LaunchDaemon %s uninstalled\n", cfg.Daemon.Label)

	return nil
}

// Status shows the current daemon status
func Status(log *zap.Logger, cfg *config.Config, out io.Writer) error {
	plistPath := cfg.Daemon.PlistPath

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Fprintf(out, "LaunchDaemon %s is not installed\n", cfg.Daemon.Label)
		return nil
	}

	fmt.Fprintf(out, "LaunchDaemon: %s\n", cfg.Daemon.Label)
	fmt.Fprintf(out, "Plist path: %s\n", plistPath)

	// Check if loaded
	cmd := exec.Command("launchctl", "list", cfg.Daemon.Label)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintln(out, "Status: Not loaded")
	} else {
		fmt.Fprintln(out, "Status: Loaded")
		fmt.Fprintf(out, "\n%s", string(output))
	}

	// Check stdout/stderr files
	stdoutPath := filepath.Join(cfg.Options.WorkingDirectory, "stdout")
	stderrPath := filepath.Join(cfg.Options.WorkingDirectory, "stderr")

	if info, err := os.Stat(stdoutPath); err == nil {
		fmt.Fprintf(out, "\nStdout log: %s (%d bytes)\n", stdoutPath, info.Size())
	}
	if info, err := os.Stat(stderrPath); err == nil {
		fmt.Fprintf(out, "Stderr log: %s (%d bytes)\n", stderrPath, info.Size())
	}

	return nil
}
