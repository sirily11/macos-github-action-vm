package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rxtech-lab/rvmm/internal/config"
	"go.uber.org/zap"
)

// RegistrationTokenResponse represents the GitHub API response
type RegistrationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GitHubClient handles GitHub API interactions
type GitHubClient struct {
	cfg    *config.Config
	log    *zap.Logger
	client *http.Client
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(cfg *config.Config, log *zap.Logger) *GitHubClient {
	return &GitHubClient{
		cfg: cfg,
		log: log,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetRegistrationToken requests a new runner registration token from GitHub
func (g *GitHubClient) GetRegistrationToken() (string, error) {
	g.log.Info("Requesting registration token from GitHub")

	req, err := http.NewRequest("POST", g.cfg.GitHub.RegistrationEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.cfg.GitHub.APIToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp RegistrationTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	g.log.Info("Registration token obtained",
		zap.Time("expires_at", tokenResp.ExpiresAt),
	)

	return tokenResp.Token, nil
}
