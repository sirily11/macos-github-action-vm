package posthog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rxtech-lab/rvmm/internal/config"
	"go.uber.org/zap"
)

// Event represents a PostHog event payload
type Event struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	Properties map[string]interface{} `json:"properties"`
	Timestamp  string                 `json:"timestamp"`
}

// CaptureRequest represents the PostHog capture API request
type CaptureRequest struct {
	APIKey     string                 `json:"api_key"`
	Event      string                 `json:"event"`
	Properties map[string]interface{} `json:"properties"`
	Timestamp  string                 `json:"timestamp"`
}

// Client handles PostHog API interactions
type Client struct {
	cfg        *config.PostHogConfig
	log        *zap.Logger
	httpClient *http.Client
	endpoint   string
}

// NewClient creates a new PostHog client
func NewClient(cfg *config.PostHogConfig, log *zap.Logger) *Client {
	endpoint := cfg.Host + "/capture/"

	return &Client{
		cfg: cfg,
		log: log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoint: endpoint,
	}
}

// CaptureLogEvent sends a log line to PostHog
func (c *Client) CaptureLogEvent(logType string, logLine string) error {
	event := CaptureRequest{
		APIKey: c.cfg.APIKey,
		Event:  "mac_ci_log_line",
		Properties: map[string]interface{}{
			"mac_ci_machine_label": c.cfg.MachineLabel,
			"mac_ci_log_type":      logType,
			"mac_ci_log_line":      logLine,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PostHog API error (status %d): %s", resp.StatusCode, string(body))
	}

	c.log.Debug("Log event sent to PostHog",
		zap.String("log_type", logType),
		zap.String("machine_label", c.cfg.MachineLabel),
	)

	return nil
}

// CaptureLogEventBatch sends multiple log lines to PostHog in a batch
func (c *Client) CaptureLogEventBatch(logType string, logLines []string) error {
	if len(logLines) == 0 {
		return nil
	}

	// PostHog batch endpoint expects an array of events
	batchEndpoint := c.cfg.Host + "/batch/"

	events := make([]CaptureRequest, 0, len(logLines))
	timestamp := time.Now().UTC().Format(time.RFC3339)

	for _, line := range logLines {
		events = append(events, CaptureRequest{
			APIKey: c.cfg.APIKey,
			Event:  "mac_ci_log_line",
			Properties: map[string]interface{}{
				"mac_ci_machine_label": c.cfg.MachineLabel,
				"mac_ci_log_type":      logType,
				"mac_ci_log_line":      line,
			},
			Timestamp: timestamp,
		})
	}

	payload := map[string]interface{}{
		"api_key": c.cfg.APIKey,
		"batch":   events,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	req, err := http.NewRequest("POST", batchEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PostHog API error (status %d): %s", resp.StatusCode, string(body))
	}

	c.log.Debug("Batch log events sent to PostHog",
		zap.String("log_type", logType),
		zap.Int("count", len(logLines)),
		zap.String("machine_label", c.cfg.MachineLabel),
	)

	return nil
}
