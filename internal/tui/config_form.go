package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rxtech-lab/rvmm/internal/config"
)

type configField struct {
	key      string
	label    string
	required bool
	secret   bool
}

type configForm struct {
	fields     []configField
	inputs     []textinput.Model
	focusIndex int
	errMsg     string
}

func newConfigForm(cfg *config.Config) configForm {
	fields := []configField{
		{key: "github.api_token", label: "GitHub API token", required: true, secret: true},
		{key: "github.registration_endpoint", label: "Registration endpoint", required: true},
		{key: "github.runner_url", label: "Runner URL", required: true},
		{key: "github.runner_name", label: "Runner name"},
		{key: "github.runner_labels", label: "Runner labels (comma)"},
		{key: "github.runner_group", label: "Runner group (optional)"},
		{key: "vm.username", label: "VM username", required: true},
		{key: "vm.password", label: "VM password", required: true, secret: true},
		{key: "registry.url", label: "Registry URL"},
		{key: "registry.image_name", label: "Registry image name", required: true},
		{key: "registry.username", label: "Registry username"},
		{key: "registry.password", label: "Registry password", secret: true},
		{key: "options.log_file", label: "Log file"},
		{key: "options.max_concurrent_runners", label: "Max concurrent runners"},
		{key: "options.shutdown_flag_file", label: "Shutdown flag file"},
		{key: "options.working_directory", label: "Working directory"},
		{key: "daemon.label", label: "Daemon label"},
		{key: "daemon.plist_path", label: "Daemon plist path"},
		{key: "daemon.user", label: "Daemon user"},
	}

	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		input := textinput.New()
		input.CharLimit = 512
		input.Width = 50
		input.SetValue(getFieldValue(cfg, field.key))
		if field.secret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		inputs[i] = input
	}

	if len(inputs) > 0 {
		inputs[0].Focus()
	}

	return configForm{fields: fields, inputs: inputs, focusIndex: 0}
}

func (f configForm) Update(msg tea.Msg) (configForm, tea.Cmd) {
	var cmd tea.Cmd
	for i := range f.inputs {
		if i == f.focusIndex {
			f.inputs[i], cmd = f.inputs[i].Update(msg)
			return f, cmd
		}
	}
	return f, nil
}

func (f configForm) updateFocus(key string) configForm {
	f.inputs[f.focusIndex].Blur()
	switch key {
	case "tab", "down":
		f.focusIndex++
		if f.focusIndex >= len(f.inputs) {
			f.focusIndex = 0
		}
	case "shift+tab", "up":
		f.focusIndex--
		if f.focusIndex < 0 {
			f.focusIndex = len(f.inputs) - 1
		}
	}
	f.inputs[f.focusIndex].Focus()
	return f
}

func (f configForm) toConfig() (*config.Config, error) {
	cfg := defaultConfig()
	for i, field := range f.fields {
		value := strings.TrimSpace(f.inputs[i].Value())
		if field.required && value == "" {
			return nil, fmt.Errorf("%s is required", field.label)
		}
		setFieldValue(cfg, field.key, value)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getFieldValue(cfg *config.Config, key string) string {
	switch key {
	case "github.api_token":
		return cfg.GitHub.APIToken
	case "github.registration_endpoint":
		return cfg.GitHub.RegistrationEndpoint
	case "github.runner_url":
		return cfg.GitHub.RunnerURL
	case "github.runner_name":
		return cfg.GitHub.RunnerName
	case "github.runner_labels":
		return strings.Join(cfg.GitHub.RunnerLabels, ",")
	case "github.runner_group":
		return cfg.GitHub.RunnerGroup
	case "vm.username":
		return cfg.VM.Username
	case "vm.password":
		return cfg.VM.Password
	case "registry.url":
		return cfg.Registry.URL
	case "registry.image_name":
		return cfg.Registry.ImageName
	case "registry.username":
		return cfg.Registry.Username
	case "registry.password":
		return cfg.Registry.Password
	case "options.log_file":
		return cfg.Options.LogFile
	case "options.max_concurrent_runners":
		return strconv.Itoa(cfg.Options.MaxConcurrentRunners)
	case "options.shutdown_flag_file":
		return cfg.Options.ShutdownFlagFile
	case "options.working_directory":
		return cfg.Options.WorkingDirectory
	case "daemon.label":
		return cfg.Daemon.Label
	case "daemon.plist_path":
		return cfg.Daemon.PlistPath
	case "daemon.user":
		return cfg.Daemon.User
	default:
		return ""
	}
}

func setFieldValue(cfg *config.Config, key, value string) {
	switch key {
	case "github.api_token":
		cfg.GitHub.APIToken = value
	case "github.registration_endpoint":
		cfg.GitHub.RegistrationEndpoint = value
	case "github.runner_url":
		cfg.GitHub.RunnerURL = value
	case "github.runner_name":
		if value != "" {
			cfg.GitHub.RunnerName = value
		}
	case "github.runner_labels":
		if value != "" {
			cfg.GitHub.RunnerLabels = splitCSV(value)
		}
	case "github.runner_group":
		cfg.GitHub.RunnerGroup = value
	case "vm.username":
		cfg.VM.Username = value
	case "vm.password":
		cfg.VM.Password = value
	case "registry.url":
		cfg.Registry.URL = value
	case "registry.image_name":
		cfg.Registry.ImageName = value
	case "registry.username":
		cfg.Registry.Username = value
	case "registry.password":
		cfg.Registry.Password = value
	case "options.log_file":
		if value != "" {
			cfg.Options.LogFile = value
		}
	case "options.max_concurrent_runners":
		if value != "" {
			if n, err := strconv.Atoi(value); err == nil && n >= 1 {
				cfg.Options.MaxConcurrentRunners = n
			}
		}
	case "options.shutdown_flag_file":
		if value != "" {
			cfg.Options.ShutdownFlagFile = value
		}
	case "options.working_directory":
		if value != "" {
			cfg.Options.WorkingDirectory = value
		}
	case "daemon.label":
		if value != "" {
			cfg.Daemon.Label = value
		}
	case "daemon.plist_path":
		if value != "" {
			cfg.Daemon.PlistPath = value
		}
	case "daemon.user":
		if value != "" {
			cfg.Daemon.User = value
		}
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return labels
}
