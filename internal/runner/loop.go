package runner

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/setup"
	"go.uber.org/zap"
)

// Run starts the main runner loop
func Run(ctx context.Context, log *zap.Logger, cfg *config.Config) error {
	// Check dependencies
	if err := setup.CheckDependencies(); err != nil {
		return err
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Info("Received signal, shutting down", zap.String("signal", sig.String()))
		cancel()
	}()

	// Create managers
	vm := NewVMManager(cfg, log)
	github := NewGitHubClient(cfg, log)

	// Login to registry if needed
	if err := vm.Login(ctx); err != nil {
		return fmt.Errorf("registry login failed: %w", err)
	}

	log.Info("Starting runner loop")

	// Main loop
	for {
		select {
		case <-ctx.Done():
			log.Info("Runner loop stopped")
			return nil
		default:
		}

		// Check shutdown flag
		if cfg.Options.ShutdownFlagFile != "" {
			if _, err := os.Stat(cfg.Options.ShutdownFlagFile); err == nil {
				log.Info("Shutdown flag file detected, exiting")
				return nil
			}
		}

		// Ensure image is cached
		exists, err := vm.ImageExists(ctx)
		if err != nil {
			log.Error("Failed to check image", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		if !exists {
			log.Info("Image not found locally, pulling from registry")
			if cfg.Registry.URL == "" {
				return fmt.Errorf("image not found and no registry URL configured")
			}

			if err := vm.PullImage(ctx); err != nil {
				log.Error("Failed to pull image", zap.Error(err))
				time.Sleep(30 * time.Second)
				continue
			}
		}

		// Run one iteration
		if err := runOnce(ctx, log, cfg, vm, github); err != nil {
			if ctx.Err() != nil {
				// Context cancelled, exit gracefully
				return nil
			}
			log.Error("Run iteration failed", zap.Error(err))
			time.Sleep(10 * time.Second)
		}
	}
}

func runOnce(ctx context.Context, log *zap.Logger, cfg *config.Config, vm *VMManager, github *GitHubClient) error {
	runID := generateRunID()
	log = log.With(zap.String("run_id", runID))

	log.Info("Starting new run")

	// Get registration token
	token, err := github.GetRegistrationToken()
	if err != nil {
		return fmt.Errorf("failed to get registration token: %w", err)
	}

	// Generate instance name
	instanceName := fmt.Sprintf("runner_%s_%s", cfg.GitHub.RunnerName, runID)

	// Ensure cleanup happens
	defer vm.Cleanup(ctx, instanceName)

	// Clone VM
	if err := vm.Clone(ctx, instanceName); err != nil {
		return fmt.Errorf("failed to clone VM: %w", err)
	}

	// Start VM
	vmCmd, err := vm.Start(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Wait for the VM process in the background
	vmDone := make(chan error, 1)
	go func() {
		vmDone <- vmCmd.Wait()
	}()

	// Wait for IP
	ip, err := vm.WaitForIP(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("failed to get VM IP: %w", err)
	}

	// Create SSH client
	ssh := NewSSHClient(cfg, log)

	// Wait for SSH
	if err := ssh.WaitForSSH(ctx, ip); err != nil {
		return fmt.Errorf("SSH not available: %w", err)
	}

	// Configure runner
	if err := ssh.ConfigureRunner(ctx, ip, token, instanceName); err != nil {
		return fmt.Errorf("failed to configure runner: %w", err)
	}

	// Run the runner (blocks until job completes or runner exits)
	log.Info("Runner started, waiting for job")
	if err := ssh.RunRunner(ctx, ip); err != nil {
		// Runner exit is expected after job completion
		log.Info("Runner exited", zap.Error(err))
	}

	// Stop VM
	log.Info("Stopping VM")
	if err := vm.Stop(ctx, instanceName); err != nil {
		log.Warn("Failed to stop VM gracefully", zap.Error(err))
	}

	// Wait for VM process to exit
	select {
	case <-vmDone:
	case <-time.After(30 * time.Second):
		log.Warn("VM process did not exit in time")
	}

	log.Info("Run completed successfully")
	return nil
}

func generateRunID() string {
	return fmt.Sprintf("%d%d", rand.Intn(10000), rand.Intn(10000))
}
