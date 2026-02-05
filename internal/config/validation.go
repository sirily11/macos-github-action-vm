package config

import (
	"errors"
	"net/url"
	"strings"
)

// Validate checks that all required configuration fields are present
func (c *Config) Validate() error {
	var errs []string

	// GitHub validation
	if c.GitHub.APIToken == "" {
		errs = append(errs, "github.api_token is required")
	}
	if c.GitHub.RegistrationEndpoint == "" {
		errs = append(errs, "github.registration_endpoint is required")
	} else if _, err := url.Parse(c.GitHub.RegistrationEndpoint); err != nil {
		errs = append(errs, "github.registration_endpoint must be a valid URL")
	}
	if c.GitHub.RunnerURL == "" {
		errs = append(errs, "github.runner_url is required")
	}

	// Registry validation
	if c.Registry.ImageName == "" {
		errs = append(errs, "registry.image_name is required")
	}

	// VM validation
	if c.VM.Username == "" {
		errs = append(errs, "vm.username is required")
	}
	if c.VM.Password == "" {
		errs = append(errs, "vm.password is required")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
