package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/selfhosters-cc/container-census/internal/models"
)

// Discord embed colors by event type
const (
	ColorRed    = 16711680 // 0xFF0000 - container_stopped
	ColorGreen  = 65280    // 0x00FF00 - container_started
	ColorBlue   = 3447003  // 0x3498DB - new_image
	ColorOrange = 16744448 // 0xFF8C00 - high_cpu, high_memory
	ColorPurple = 9043969  // 0x8A00FF - anomalous_behavior
	ColorGray   = 9807270  // 0x959B9B - state_change, default
)

// DiscordChannel implements Discord webhook notifications
type DiscordChannel struct {
	name   string
	config models.DiscordConfig
	client *http.Client
}

// NewDiscordChannel creates a new Discord channel
func NewDiscordChannel(ch *models.NotificationChannel) (*DiscordChannel, error) {
	// Parse config
	configJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var discordConfig models.DiscordConfig
	if err := json.Unmarshal(configJSON, &discordConfig); err != nil {
		return nil, fmt.Errorf("failed to parse discord config: %w", err)
	}

	if discordConfig.WebhookURL == "" {
		return nil, fmt.Errorf("discord webhook URL is required")
	}

	return &DiscordChannel{
		name:   ch.Name,
		config: discordConfig,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// getColorForEvent returns the appropriate Discord embed color for an event type
func getColorForEvent(eventType string) int {
	switch eventType {
	case models.EventTypeContainerStopped:
		return ColorRed
	case models.EventTypeContainerStarted, models.EventTypeContainerResumed:
		return ColorGreen
	case models.EventTypeNewImage:
		return ColorBlue
	case models.EventTypeHighCPU, models.EventTypeHighMemory:
		return ColorOrange
	case models.EventTypeAnomalousBehavior:
		return ColorPurple
	default:
		return ColorGray
	}
}

// getTitleForEvent returns a human-readable title for an event type
func getTitleForEvent(eventType string) string {
	switch eventType {
	case models.EventTypeContainerStopped:
		return "Container Stopped"
	case models.EventTypeContainerStarted:
		return "Container Started"
	case models.EventTypeContainerPaused:
		return "Container Paused"
	case models.EventTypeContainerResumed:
		return "Container Resumed"
	case models.EventTypeNewImage:
		return "Image Updated"
	case models.EventTypeHighCPU:
		return "High CPU Usage"
	case models.EventTypeHighMemory:
		return "High Memory Usage"
	case models.EventTypeAnomalousBehavior:
		return "Anomalous Behavior Detected"
	case models.EventTypeStateChange:
		return "State Changed"
	case "test":
		return "Test Notification"
	default:
		return "Container Event"
	}
}

// Send sends a Discord notification with a rich embed
func (dc *DiscordChannel) Send(ctx context.Context, message string, event models.NotificationEvent) error {
	// Build embed fields
	fields := []map[string]interface{}{}

	if event.ContainerName != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Container",
			"value":  event.ContainerName,
			"inline": true,
		})
	}

	if event.HostName != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Host",
			"value":  event.HostName,
			"inline": true,
		})
	}

	if event.Image != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Image",
			"value":  event.Image,
			"inline": true,
		})
	}

	// Add state change info
	if event.OldState != "" && event.NewState != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "State Change",
			"value":  fmt.Sprintf("%s → %s", event.OldState, event.NewState),
			"inline": true,
		})
	}

	// Add image change info
	if event.OldImage != "" && event.NewImage != "" {
		fields = append(fields, map[string]interface{}{
			"name":   "Image Change",
			"value":  fmt.Sprintf("%s → %s", event.OldImage, event.NewImage),
			"inline": false,
		})
	}

	// Add resource usage info
	if event.CPUPercent > 0 {
		fields = append(fields, map[string]interface{}{
			"name":   "CPU Usage",
			"value":  fmt.Sprintf("%.1f%%", event.CPUPercent),
			"inline": true,
		})
	}

	if event.MemoryPercent > 0 {
		fields = append(fields, map[string]interface{}{
			"name":   "Memory Usage",
			"value":  fmt.Sprintf("%.1f%%", event.MemoryPercent),
			"inline": true,
		})
	}

	// Build the embed
	embed := map[string]interface{}{
		"title":       getTitleForEvent(event.EventType),
		"description": message,
		"color":       getColorForEvent(event.EventType),
		"timestamp":   event.Timestamp.Format(time.RFC3339),
		"footer": map[string]interface{}{
			"text": "Container Census",
		},
	}

	if len(fields) > 0 {
		embed["fields"] = fields
	}

	// Build the payload
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", dc.config.WebhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Container-Census-Notifier/1.0")

	// Send with retry logic (3 attempts)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := dc.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d failed: %w", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			// Recreate request for retry (body was consumed)
			req, _ = http.NewRequestWithContext(ctx, "POST", dc.config.WebhookURL, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "Container-Census-Notifier/1.0")
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Success
		}

		lastErr = fmt.Errorf("attempt %d: HTTP %d", attempt, resp.StatusCode)
		time.Sleep(time.Duration(attempt) * time.Second)
		// Recreate request for retry
		req, _ = http.NewRequestWithContext(ctx, "POST", dc.config.WebhookURL, bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Container-Census-Notifier/1.0")
	}

	return fmt.Errorf("discord webhook failed after 3 attempts: %w", lastErr)
}

// Test sends a test notification
func (dc *DiscordChannel) Test(ctx context.Context) error {
	testEvent := models.NotificationEvent{
		EventType:     "test",
		Timestamp:     time.Now(),
		ContainerName: "test-container",
		HostName:      "test-host",
		Image:         "test-image:latest",
	}

	return dc.Send(ctx, "Test notification from Container Census", testEvent)
}

// Type returns the channel type
func (dc *DiscordChannel) Type() string {
	return models.ChannelTypeDiscord
}

// Name returns the channel name
func (dc *DiscordChannel) Name() string {
	return dc.name
}
