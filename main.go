package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/runner"
	"github.com/rxtech-lab/rvmm/internal/tui"
	"go.uber.org/zap"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		runHeadless()
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
