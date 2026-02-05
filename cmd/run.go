package cmd

import (
	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/runner"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the runner loop",
	Long: `Start the main runner loop that manages VM lifecycle.

This command will:
  1. Load configuration from the specified file
  2. Check for shutdown flag file
  3. Pull/cache VM image from registry
  4. Get GitHub registration token
  5. Clone and boot VM
  6. Configure and run GitHub Actions runner
  7. Cleanup after job completion
  8. Loop back to step 2

Use Ctrl+C to gracefully shutdown the runner.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := GetLogger()

		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			log.Error("Failed to load configuration", zap.Error(err))
			return err
		}

		if err := cfg.Validate(); err != nil {
			log.Error("Invalid configuration", zap.Error(err))
			return err
		}

		log.Info("Starting runner",
			zap.String("runner_name", cfg.GitHub.RunnerName),
			zap.String("image", cfg.Registry.ImageName),
		)

		return runner.Run(cmd.Context(), log, cfg)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
