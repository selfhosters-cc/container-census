package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// NtfyChannel implements ntfy.sh notifications
type NtfyChannel struct {
	name   string
	config models.NtfyConfig
	client *http.Client
}

// NewNtfyChannel creates a new ntfy channel
func NewNtfyChannel(ch *models.NotificationChannel) (*NtfyChannel, error) {
	// Parse config
	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var ntfyConfig models.NtfyConfig
	if err := json.Unmarshal(configJSON, &ntfyConfig); err != nil {
		return nil, fmt.Errorf("failed to parse ntfy config: %w", err)
	}

	// Default to ntfy.sh if no server specified
	if ntfyConfig.ServerURL == "" {
		ntfyConfig.ServerURL = "https://ntfy.sh"
	}

	// Remove trailing slash
	ntfyConfig.ServerURL = strings.TrimSuffix(ntfyConfig.ServerURL, "/")

	if ntfyConfig.Topic == "" {
		return nil, fmt.Errorf("ntfy topic is required")
	}

	return &NtfyChannel{
		name:   ch.Name,
		config: ntfyConfig,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Send sends an ntfy notification
func (nc *NtfyChannel) Send(ctx context.Context, message string, event models.NotificationEvent) error {
	// Build ntfy message
	ntfyMsg := map[string]interface{}{
		"topic":   nc.config.Topic,
		"message": message,
		"title":   "Container Census",
	}

	// Set priority based on event type
	priority := nc.getPriority(event.EventType)
	ntfyMsg["priority"] = priority

	// Add tags based on event type
	tags := nc.getTags(event.EventType)
	if len(tags) > 0 {
		ntfyMsg["tags"] = tags
	}

	// Add actions/click URL if applicable
	// (Could link back to Census UI showing the container)

	payloadBytes, err := json.Marshal(ntfyMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal ntfy message: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s", nc.config.ServerURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Container-Census-Notifier/1.0")

	// Add authentication if token provided
	if nc.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+nc.config.Token)
	}

	// Send with retry logic (3 attempts)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := nc.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d failed: %w", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Success
		}

		lastErr = fmt.Errorf("attempt %d: HTTP %d", attempt, resp.StatusCode)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	return fmt.Errorf("ntfy failed after 3 attempts: %w", lastErr)
}

// getPriority returns ntfy priority based on event type
func (nc *NtfyChannel) getPriority(eventType string) int {
	switch eventType {
	case models.EventTypeContainerStopped:
		return 4 // High
	case models.EventTypeHighCPU, models.EventTypeHighMemory:
		return 4 // High
	case models.EventTypeAnomalousBehavior:
		return 4 // High
	case models.EventTypeNewImage:
		return 3 // Default
	case models.EventTypeContainerStarted:
		return 3 // Default
	default:
		return 3 // Default
	}
}

// getTags returns ntfy tags based on event type
func (nc *NtfyChannel) getTags(eventType string) []string {
	switch eventType {
	case models.EventTypeContainerStarted:
		return []string{"white_check_mark"}
	case models.EventTypeContainerStopped:
		return []string{"octagonal_sign"}
	case models.EventTypeNewImage:
		return []string{"arrows_counterclockwise"}
	case models.EventTypeHighCPU, models.EventTypeHighMemory:
		return []string{"warning"}
	case models.EventTypeAnomalousBehavior:
		return []string{"mag"}
	default:
		return []string{"information_source"}
	}
}

// Test sends a test notification
func (nc *NtfyChannel) Test(ctx context.Context) error {
	testEvent := models.NotificationEvent{
		EventType:     "test",
		Timestamp:     time.Now(),
		ContainerName: "test-container",
		HostName:      "test-host",
	}

	return nc.Send(ctx, "🧪 Test notification from Container Census", testEvent)
}

// Type returns the channel type
func (nc *NtfyChannel) Type() string {
	return models.ChannelTypeNtfy
}

// Name returns the channel name
func (nc *NtfyChannel) Name() string {
	return nc.name
}
