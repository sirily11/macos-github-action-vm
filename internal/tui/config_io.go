package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rxtech-lab/rvmm/internal/config"
	"gopkg.in/yaml.v3"
)

func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config not found: %s", path)
		}
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadConfigOrDefault(path string) *config.Config {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg
	}

	return defaultConfig()
}

func defaultConfig() *config.Config {
	return &config.Config{
		GitHub: config.GitHubConfig{
			RunnerName:   "runner",
			RunnerLabels: []string{"self-hosted", "arm64"},
		},
		VM: config.VMConfig{
			Username: "admin",
			Password: "admin",
		},
		Options: config.OptionsConfig{
			LogFile:          "runner.log",
			ShutdownFlagFile: ".shutdown",
			WorkingDirectory: "/Users/admin/vm",
		},
		Daemon: config.DaemonConfig{
			Label:     "com.mirego.ekiden",
			PlistPath: "/Library/LaunchDaemons/com.mirego.ekiden.plist",
			User:      "admin",
		},
	}
}

func defaultConfigPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "rvmm.yaml"
	}
	return filepath.Join(workingDir, "rvmm.yaml")
}

func writeConfig(path string, cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}
