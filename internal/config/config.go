package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config represents the full configuration structure
type Config struct {
	GitHub   GitHubConfig   `mapstructure:"github" yaml:"github"`
	VM       VMConfig       `mapstructure:"vm" yaml:"vm"`
	Registry RegistryConfig `mapstructure:"registry" yaml:"registry"`
	Options  OptionsConfig  `mapstructure:"options" yaml:"options"`
	Daemon   DaemonConfig   `mapstructure:"daemon" yaml:"daemon"`
}

// GitHubConfig contains GitHub API and runner settings
type GitHubConfig struct {
	APIToken             string   `mapstructure:"api_token" yaml:"api_token"`
	RegistrationEndpoint string   `mapstructure:"registration_endpoint" yaml:"registration_endpoint"`
	RunnerURL            string   `mapstructure:"runner_url" yaml:"runner_url"`
	RunnerName           string   `mapstructure:"runner_name" yaml:"runner_name"`
	RunnerLabels         []string `mapstructure:"runner_labels" yaml:"runner_labels"`
}

// VMConfig contains VM credentials
type VMConfig struct {
	Username string `mapstructure:"username" yaml:"username"`
	Password string `mapstructure:"password" yaml:"password"`
}

// RegistryConfig contains OCI registry settings
type RegistryConfig struct {
	URL       string `mapstructure:"url" yaml:"url"`
	ImageName string `mapstructure:"image_name" yaml:"image_name"`
	Username  string `mapstructure:"username" yaml:"username"`
	Password  string `mapstructure:"password" yaml:"password"`
}

// OptionsConfig contains runtime options
type OptionsConfig struct {
	TruncateSize     string `mapstructure:"truncate_size" yaml:"truncate_size"`
	LogFile          string `mapstructure:"log_file" yaml:"log_file"`
	ShutdownFlagFile string `mapstructure:"shutdown_flag_file" yaml:"shutdown_flag_file"`
	WorkingDirectory string `mapstructure:"working_directory" yaml:"working_directory"`
}

// DaemonConfig contains LaunchDaemon settings
type DaemonConfig struct {
	Label     string `mapstructure:"label" yaml:"label"`
	PlistPath string `mapstructure:"plist_path" yaml:"plist_path"`
	User      string `mapstructure:"user" yaml:"user"`
}

// Load reads configuration from file with defaults
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("rvmm")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.rvmm")
		v.AddConfigPath("/etc/rvmm")
	}

	// Read config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Unmarshal
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// VM defaults
	v.SetDefault("vm.username", "admin")
	v.SetDefault("vm.password", "admin")

	// GitHub defaults
	v.SetDefault("github.runner_name", "runner")
	v.SetDefault("github.runner_labels", []string{"self-hosted", "arm64"})

	// Options defaults
	v.SetDefault("options.log_file", "runner.log")
	v.SetDefault("options.shutdown_flag_file", ".shutdown")
	v.SetDefault("options.working_directory", "/Users/admin/vm")

	// Daemon defaults
	v.SetDefault("daemon.label", "com.mirego.ekiden")
	v.SetDefault("daemon.plist_path", "/Library/LaunchDaemons/com.mirego.ekiden.plist")
	v.SetDefault("daemon.user", "admin")
}
