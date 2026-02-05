package cmd

import (
	"fmt"
	"os"

	"github.com/rxtech-lab/rvmm/assets"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	outputFile string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate sample configuration file",
	Long: `Generate a sample YAML configuration file with all available options.

The generated file includes comments explaining each option.
Copy and edit this file for your deployment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVarP(&outputFile, "output", "o", "ekiden.yaml", "Output file path")
}

func runConfig() error {
	log := GetLogger()

	// Check if file exists
	if _, err := os.Stat(outputFile); err == nil {
		return fmt.Errorf("file %s already exists, use a different name or remove it first", outputFile)
	}

	// Write sample config
	if err := os.WriteFile(outputFile, assets.ConfigExample, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Info("Sample configuration written", zap.String("file", outputFile))
	fmt.Printf("Sample configuration written to %s\n", outputFile)
	fmt.Println("Edit this file with your settings, then run:")
	fmt.Printf("  ekiden run --config %s\n", outputFile)

	return nil
}
