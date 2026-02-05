package cmd

import (
	"github.com/rxtech-lab/rvmm/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "One-time host machine setup",
	Long: `Perform initial setup on the host machine.

This command will:
  - Install Homebrew if not present
  - Install required packages: tart, sshpass, wget
  - Validate macOS settings

Run this command once on a new host before running the runner.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return setup.Run(GetLogger())
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
