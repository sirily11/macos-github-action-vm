package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rxtech-lab/rvmm/internal/config"
	"go.uber.org/zap"
)

// SSHClient handles SSH command execution on VMs
type SSHClient struct {
	cfg *config.Config
	log *zap.Logger
}

// NewSSHClient creates a new SSH client
func NewSSHClient(cfg *config.Config, log *zap.Logger) *SSHClient {
	return &SSHClient{
		cfg: cfg,
		log: log,
	}
}

// WaitForSSH polls until SSH is available on the VM
func (s *SSHClient) WaitForSSH(ctx context.Context, ip string) error {
	s.log.Info("Waiting for SSH to be available", zap.String("ip", ip))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for SSH")
		case <-ticker.C:
			cmd := exec.CommandContext(ctx, "sshpass", "-e", "ssh",
				"-q",
				"-o", "ConnectTimeout=1",
				"-o", "StrictHostKeyChecking=no",
				fmt.Sprintf("%s@%s", s.cfg.VM.Username, ip),
				"pwd",
			)
			cmd.Env = append(os.Environ(), "SSHPASS="+s.cfg.VM.Password)

			if err := cmd.Run(); err == nil {
				s.log.Info("SSH is available")
				return nil
			}
		}
	}
}

// Execute runs a command on the VM via SSH
func (s *SSHClient) Execute(ctx context.Context, ip string, command string, showOutput bool) error {
	s.log.Debug("Executing SSH command",
		zap.String("ip", ip),
		zap.String("command", command),
	)

	cmd := exec.CommandContext(ctx, "sshpass", "-e", "ssh",
		"-q",
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", s.cfg.VM.Username, ip),
		command,
	)
	cmd.Env = append(os.Environ(), "SSHPASS="+s.cfg.VM.Password)

	if showOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SSH command failed: %w", err)
	}

	return nil
}

// ExecuteWithOutput runs a command and returns the output
func (s *SSHClient) ExecuteWithOutput(ctx context.Context, ip string, command string) (string, error) {
	s.log.Debug("Executing SSH command",
		zap.String("ip", ip),
		zap.String("command", command),
	)

	cmd := exec.CommandContext(ctx, "sshpass", "-e", "ssh",
		"-q",
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", s.cfg.VM.Username, ip),
		command,
	)
	cmd.Env = append(os.Environ(), "SSHPASS="+s.cfg.VM.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("SSH command failed: %w", err)
	}

	return string(output), nil
}

// ConfigureRunner sets up the GitHub Actions runner on the VM
func (s *SSHClient) ConfigureRunner(ctx context.Context, ip string, token string) error {
	s.log.Info("Configuring GitHub Actions runner")

	labels := s.cfg.GitHub.RunnerLabels
	if len(labels) == 0 {
		labels = []string{"self-hosted"}
	}

	// Build label string
	labelsStr := ""
	for i, l := range labels {
		if i > 0 {
			labelsStr += ","
		}
		labelsStr += l
	}

	configCmd := fmt.Sprintf(
		"./actions-runner/config.sh --url %s --token %s --ephemeral --name %s --labels %s --unattended --replace",
		s.cfg.GitHub.RunnerURL,
		token,
		s.cfg.GitHub.RunnerName,
		labelsStr,
	)

	return s.Execute(ctx, ip, configCmd, false)
}

// RunRunner starts the GitHub Actions runner and waits for completion
func (s *SSHClient) RunRunner(ctx context.Context, ip string) error {
	s.log.Info("Starting GitHub Actions runner")

	// Source profile and run
	runCmd := "source ~/.zprofile && ./actions-runner/run.sh"

	return s.Execute(ctx, ip, runCmd, true)
}
