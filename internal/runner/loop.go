package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
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

	// Create shared GitHub client (thread-safe)
	github := NewGitHubClient(cfg, log)

	// Initialize image once before starting workers
	var initOnce sync.Once
	var initErr error
	initImage := func() {
		initOnce.Do(func() {
			log.Info("Initializing VM image")
			vm := NewVMManager(cfg, log)

			// Login to registry if needed
			if err := vm.Login(ctx); err != nil {
				initErr = fmt.Errorf("registry login failed: %w", err)
				return
			}

			// Ensure image is cached
			exists, err := vm.ImageExists(ctx)
			if err != nil {
				initErr = fmt.Errorf("failed to check image: %w", err)
				return
			}

			if !exists {
				log.Info("Image not found locally, pulling from registry")
				if cfg.Registry.URL == "" {
					initErr = fmt.Errorf("image not found and no registry URL configured")
					return
				}

				if err := vm.PullImage(ctx); err != nil {
					initErr = fmt.Errorf("failed to pull image: %w", err)
					return
				}
			}
			log.Info("VM image initialized successfully")
		})
	}

	// Initialize image
	initImage()
	if initErr != nil {
		return initErr
	}

	// Create slot channel for bounded concurrency
	slots := make(chan int, cfg.Options.MaxConcurrentRunners)
	for i := 0; i < cfg.Options.MaxConcurrentRunners; i++ {
		slots <- i
	}

	// WaitGroup to track active workers
	var wg sync.WaitGroup

	log.Info("Starting runner loop",
		zap.Int("max_concurrent_runners", cfg.Options.MaxConcurrentRunners),
	)

	// Main dispatch loop
	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled, waiting for active runners to complete")
			wg.Wait()
			log.Info("All runners stopped")
			return nil
		default:
		}

		// Check shutdown flag
		if cfg.Options.ShutdownFlagFile != "" {
			if _, err := os.Stat(cfg.Options.ShutdownFlagFile); err == nil {
				log.Info("Shutdown flag file detected, waiting for active runners")
				wg.Wait()
				return nil
			}
		}

		// Acquire a slot (blocks if all slots are in use)
		var slotID int
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case slotID = <-slots:
			// Got a slot, continue
		}

		// Launch worker
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			defer func() {
				// Return slot to pool
				slots <- slot
			}()

			// Create per-worker logger
			workerLog := log.With(zap.Int("slot_id", slot))
			workerLog.Info("Worker starting")

			// Create per-worker VM manager to avoid race conditions
			vm := NewVMManager(cfg, workerLog)

			// Run one iteration
			if err := runOnce(ctx, workerLog, cfg, vm, github, slot); err != nil {
				if ctx.Err() != nil {
					// Context cancelled, exit gracefully
					workerLog.Info("Worker stopped due to context cancellation")
					return
				}
				workerLog.Error("Worker run failed", zap.Error(err))
				// Brief delay before slot is returned
				time.Sleep(10 * time.Second)
			} else {
				workerLog.Info("Worker completed successfully")
			}
		}(slotID)
	}
}

func runOnce(ctx context.Context, log *zap.Logger, cfg *config.Config, vm *VMManager, github *GitHubClient, slotID int) error {
	log.Info("Starting new run")

	// Get registration token
	token, err := github.GetRegistrationToken()
	if err != nil {
		return fmt.Errorf("failed to get registration token: %w", err)
	}

	// Generate instance name using slot ID
	instanceName := fmt.Sprintf("%s_%d", cfg.GitHub.RunnerName, slotID)

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
