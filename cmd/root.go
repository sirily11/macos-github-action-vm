package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Global flags
	verbose    bool
	configFile string

	// Logger instance
	logger *zap.Logger
)

var rootCmd = &cobra.Command{
	Use:   "ekiden",
	Short: "Ekiden CLI - macOS VM runner for GitHub Actions",
	Long: `Ekiden is a CLI tool for managing ephemeral macOS virtual machines
as GitHub Actions self-hosted runners.

It automates the entire lifecycle:
  - Pull VM images from OCI registries
  - Clone and boot VMs using Tart
  - Register runners with GitHub
  - Execute jobs and cleanup

Use "ekiden [command] --help" for more information about a command.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initLogger()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logger != nil {
			_ = logger.Sync()
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file (default: rvmm.yaml)")
}

func initLogger() error {
	var config zap.Config

	if verbose {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "time"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	var err error
	logger, err = config.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	if logger == nil {
		// Return a no-op logger if not initialized
		return zap.NewNop()
	}
	return logger
}

// GetConfigFile returns the config file path from global flag
func GetConfigFile() string {
	return configFile
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// SetConfigFileForTest allows tests to set the config file
func SetConfigFileForTest(path string) {
	configFile = path
}

func checkError(err error) {
	if err != nil {
		GetLogger().Error("Command failed", zap.Error(err))
		os.Exit(1)
	}
}
