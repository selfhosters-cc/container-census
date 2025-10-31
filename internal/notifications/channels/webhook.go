package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// WebhookChannel implements webhook notifications
type WebhookChannel struct {
	name   string
	config models.WebhookConfig
	client *http.Client
}

// NewWebhookChannel creates a new webhook channel
func NewWebhookChannel(ch *models.NotificationChannel) (*WebhookChannel, error) {
	// Parse config
	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var webhookConfig models.WebhookConfig
	if err := json.Unmarshal(configJSON, &webhookConfig); err != nil {
		return nil, fmt.Errorf("failed to parse webhook config: %w", err)
	}

	if webhookConfig.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	return &WebhookChannel{
		name:   ch.Name,
		config: webhookConfig,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Send sends a webhook notification
func (wc *WebhookChannel) Send(ctx context.Context, message string, event models.NotificationEvent) error {
	// Build payload
	payload := map[string]interface{}{
		"message":        message,
		"event_type":     event.EventType,
		"timestamp":      event.Timestamp.Format(time.RFC3339),
		"container_id":   event.ContainerID,
		"container_name": event.ContainerName,
		"host_id":        event.HostID,
		"host_name":      event.HostName,
		"image":          event.Image,
	}

	// Add optional fields
	if event.OldState != "" {
		payload["old_state"] = event.OldState
	}
	if event.NewState != "" {
		payload["new_state"] = event.NewState
	}
	if event.OldImage != "" {
		payload["old_image"] = event.OldImage
	}
	if event.NewImage != "" {
		payload["new_image"] = event.NewImage
	}
	if event.CPUPercent > 0 {
		payload["cpu_percent"] = event.CPUPercent
	}
	if event.MemoryPercent > 0 {
		payload["memory_percent"] = event.MemoryPercent
	}
	if len(event.Metadata) > 0 {
		payload["metadata"] = event.Metadata
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", wc.config.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Container-Census-Notifier/1.0")

	// Add custom headers
	for key, value := range wc.config.Headers {
		req.Header.Set(key, value)
	}

	// Send with retry logic (3 attempts)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := wc.client.Do(req)
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

	return fmt.Errorf("webhook failed after 3 attempts: %w", lastErr)
}

// Test sends a test notification
func (wc *WebhookChannel) Test(ctx context.Context) error {
	testEvent := models.NotificationEvent{
		EventType:     "test",
		Timestamp:     time.Now(),
		ContainerName: "test-container",
		HostName:      "test-host",
		Image:         "test-image:latest",
	}

	return wc.Send(ctx, "🧪 Test notification from Container Census", testEvent)
}

// Type returns the channel type
func (wc *WebhookChannel) Type() string {
	return models.ChannelTypeWebhook
}

// Name returns the channel name
func (wc *WebhookChannel) Name() string {
	return wc.name
}
