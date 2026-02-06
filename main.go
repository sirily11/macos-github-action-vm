package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/monitor"
	"github.com/rxtech-lab/rvmm/internal/posthog"
	"github.com/rxtech-lab/rvmm/internal/runner"
	"github.com/rxtech-lab/rvmm/internal/tui"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		runHeadless()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		monitorHeadless()
		return
	}
	tui.Run()
}

func runHeadless() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid config", zap.Error(err))
	}

	logger.Info("Starting runner in headless mode")
	if err := runner.Run(context.Background(), logger, cfg); err != nil {
		logger.Fatal("Runner exited with error", zap.Error(err))
	}
}

func monitorHeadless() {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid config", zap.Error(err))
	}

	if !cfg.PostHog.Enabled {
		logger.Fatal("PostHog is not enabled in config")
	}

	logger.Info("Starting log monitor",
		zap.String("machine_label", cfg.PostHog.MachineLabel),
		zap.String("posthog_host", cfg.PostHog.Host),
	)

	// Create PostHog client
	posthogClient := posthog.NewClient(&cfg.PostHog, logger)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create log tailers
	stdoutTailer := monitor.NewLogTailer("/Users/qiweili/rvmm/stdout", "stdout", posthogClient, logger)
	stderrTailer := monitor.NewLogTailer("/Users/qiweili/rvmm/stderr", "stderr", posthogClient, logger)

	// Start monitoring in goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := stdoutTailer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("stdout tailer error", zap.Error(err))
		}
	}()

	go func() {
		defer wg.Done()
		if err := stderrTailer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("stderr tailer error", zap.Error(err))
		}
	}()

	logger.Info("Log monitor running, press Ctrl+C to stop")

	// Wait for signal
	<-sigChan
	logger.Info("Received shutdown signal, stopping...")

	// Cancel context to stop tailers
	cancel()

	// Wait for tailers to finish
	wg.Wait()

	logger.Info("Log monitor stopped")
}
