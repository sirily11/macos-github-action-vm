package cmd

import (
	"github.com/rxtech-lab/rvmm/internal/config"
	"github.com/rxtech-lab/rvmm/internal/daemon"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage LaunchDaemon for automatic startup",
	Long: `Manage the macOS LaunchDaemon for automatic runner startup.

Subcommands:
  install   - Install and load the LaunchDaemon
  uninstall - Unload and remove the LaunchDaemon
  status    - Show current daemon status`,
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and load the LaunchDaemon",
	Long: `Install the Ekiden LaunchDaemon to start the runner automatically on boot.

This command requires sudo privileges to install to /Library/LaunchDaemons.
The daemon will be configured using the specified config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := GetLogger()

		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			return err
		}

		if err := cfg.Validate(); err != nil {
			log.Error("Invalid configuration", zap.Error(err))
			return err
		}

		return daemon.Install(log, cfg, GetConfigFile())
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Unload and remove the LaunchDaemon",
	Long:  `Unload the Ekiden LaunchDaemon and remove the plist file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := GetLogger()

		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			return err
		}

		return daemon.Uninstall(log, cfg)
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current daemon status",
	Long:  `Display the current status of the Ekiden LaunchDaemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := GetLogger()

		cfg, err := config.Load(GetConfigFile())
		if err != nil {
			return err
		}

		return daemon.Status(log, cfg)
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}
