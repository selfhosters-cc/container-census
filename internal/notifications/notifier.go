package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/notifications/channels"
	"github.com/container-census/container-census/internal/storage"
)

// NotificationService handles all notification logic
type NotificationService struct {
	db             *storage.DB
	channels       map[int64]channels.Channel
	channelsMu     sync.RWMutex
	rateLimiter    *RateLimiter
	thresholdState map[string]*ThresholdTracker // key: containerID-hostID-type
	thresholdMu    sync.RWMutex
}

// ThresholdTracker tracks threshold breach state for a container
type ThresholdTracker struct {
	BreachedAt     time.Time
	LastNotifiedAt time.Time
	BreachCount    int
}

// NewNotificationService creates a new notification service
func NewNotificationService(db *storage.DB, maxNotificationsPerHour int, batchInterval time.Duration) *NotificationService {
	ns := &NotificationService{
		db:             db,
		channels:       make(map[int64]channels.Channel),
		rateLimiter:    NewRateLimiter(maxNotificationsPerHour, batchInterval),
		thresholdState: make(map[string]*ThresholdTracker),
	}

	// Set notifier reference in rate limiter for batch sending
	ns.rateLimiter.SetNotifier(ns)

	return ns
}

// ProcessEvents is the main entry point called after each scan
func (ns *NotificationService) ProcessEvents(ctx context.Context, hostID int64) error {
	// 1. Detect lifecycle events (state changes, image updates)
	lifecycleEvents, err := ns.detectLifecycleEvents(hostID)
	if err != nil {
		return fmt.Errorf("failed to detect lifecycle events: %w", err)
	}

	// 2. Detect threshold breaches (high CPU/memory)
	thresholdEvents, err := ns.detectThresholdEvents(hostID)
	if err != nil {
		return fmt.Errorf("failed to detect threshold events: %w", err)
	}

	// 3. Detect anomalies (post-update behavior changes)
	anomalyEvents, err := ns.detectAnomalies(hostID)
	if err != nil {
		return fmt.Errorf("failed to detect anomalies: %w", err)
	}

	// Combine all events
	allEvents := append(lifecycleEvents, thresholdEvents...)
	allEvents = append(allEvents, anomalyEvents...)

	if len(allEvents) == 0 {
		return nil
	}

	log.Printf("Notification service: Processing %d events for host %d", len(allEvents), hostID)

	// 4. Match events against rules
	notifications, err := ns.matchRules(ctx, allEvents)
	if err != nil {
		return fmt.Errorf("failed to match rules: %w", err)
	}

	// 5. Apply silences
	notifications = ns.filterSilenced(notifications)

	// 6. Send notifications with rate limiting
	return ns.sendNotifications(ctx, notifications)
}

// detectLifecycleEvents detects container lifecycle events (state changes, image updates)
func (ns *NotificationService) detectLifecycleEvents(hostID int64) ([]models.NotificationEvent, error) {
	var events []models.NotificationEvent

	// Get latest containers for this host
	containers, err := ns.db.GetContainersByHost(hostID)
	if err != nil {
		return nil, err
	}

	// For each container, check for state changes and image updates
	for _, container := range containers {
		// Get lifecycle events from the last scan interval (we only care about recent changes)
		lifecycleEvents, err := ns.db.GetContainerLifecycleEvents(container.Name, hostID)
		if err != nil {
			log.Printf("Warning: Failed to get lifecycle events for %s: %v", container.Name, err)
			continue
		}

		// Get the most recent event (if any)
		if len(lifecycleEvents) > 0 {
			lastEvent := lifecycleEvents[len(lifecycleEvents)-1]

			// Only process events from the last 5 minutes to avoid re-processing old events
			if time.Since(lastEvent.Timestamp) < 5*time.Minute {
				event := ns.lifecycleEventToNotificationEvent(container, lastEvent)
				if event != nil {
					events = append(events, *event)
				}
			}
		}
	}

	return events, nil
}

// lifecycleEventToNotificationEvent converts a lifecycle event to a notification event
func (ns *NotificationService) lifecycleEventToNotificationEvent(container models.Container, le models.ContainerLifecycleEvent) *models.NotificationEvent {
	var eventType string

	switch le.EventType {
	case "started":
		eventType = models.EventTypeContainerStarted
	case "stopped":
		eventType = models.EventTypeContainerStopped
	case "paused":
		eventType = models.EventTypeContainerPaused
	case "resumed":
		eventType = models.EventTypeContainerResumed
	case "image_updated":
		eventType = models.EventTypeNewImage
	case "state_change":
		eventType = models.EventTypeStateChange
	default:
		return nil // Ignore other event types
	}

	return &models.NotificationEvent{
		EventType:     eventType,
		Timestamp:     le.Timestamp,
		ContainerID:   container.ID,
		ContainerName: container.Name,
		HostID:        container.HostID,
		HostName:      container.HostName,
		Image:         container.Image,
		OldState:      le.OldState,
		NewState:      le.NewState,
		OldImage:      le.OldImageTag,
		NewImage:      le.NewImageTag,
	}
}

// detectThresholdEvents detects CPU/memory threshold breaches
func (ns *NotificationService) detectThresholdEvents(hostID int64) ([]models.NotificationEvent, error) {
	var events []models.NotificationEvent

	// Get current containers with stats
	containers, err := ns.db.GetContainersByHost(hostID)
	if err != nil {
		return nil, err
	}

	// Get threshold duration from config (default 120 seconds)
	thresholdDuration := 120 * time.Second

	for _, container := range containers {
		if container.State != "running" {
			continue
		}

		// Check CPU threshold
		if container.CPUPercent > 0 {
			if err := ns.checkThreshold(container, "cpu", container.CPUPercent, thresholdDuration); err == nil {
				// Threshold met
				events = append(events, models.NotificationEvent{
					EventType:     models.EventTypeHighCPU,
					Timestamp:     time.Now(),
					ContainerID:   container.ID,
					ContainerName: container.Name,
					HostID:        container.HostID,
					HostName:      container.HostName,
					Image:         container.Image,
					CPUPercent:    container.CPUPercent,
				})
			}
		}

		// Check memory threshold
		if container.MemoryPercent > 0 {
			if err := ns.checkThreshold(container, "memory", container.MemoryPercent, thresholdDuration); err == nil {
				// Threshold met
				events = append(events, models.NotificationEvent{
					EventType:     models.EventTypeHighMemory,
					Timestamp:     time.Now(),
					ContainerID:   container.ID,
					ContainerName: container.Name,
					HostID:        container.HostID,
					HostName:      container.HostName,
					Image:         container.Image,
					MemoryPercent: container.MemoryPercent,
				})
			}
		}
	}

	return events, nil
}

// checkThreshold checks if a threshold has been breached for the required duration
func (ns *NotificationService) checkThreshold(container models.Container, thresholdType string, value float64, duration time.Duration) error {
	key := fmt.Sprintf("%s-%d-%s", container.ID, container.HostID, thresholdType)

	ns.thresholdMu.Lock()
	defer ns.thresholdMu.Unlock()

	tracker, exists := ns.thresholdState[key]
	if !exists {
		// First breach
		ns.thresholdState[key] = &ThresholdTracker{
			BreachedAt:  time.Now(),
			BreachCount: 1,
		}
		return fmt.Errorf("threshold just breached, duration not met")
	}

	// Check if duration met
	if time.Since(tracker.BreachedAt) < duration {
		tracker.BreachCount++
		return fmt.Errorf("threshold duration not met: %v < %v", time.Since(tracker.BreachedAt), duration)
	}

	// Duration met - return nil to indicate we should notify
	return nil
}

// ResetThreshold resets the threshold state for a container (called after notification sent)
func (ns *NotificationService) ResetThreshold(containerID string, hostID int64, thresholdType string) {
	key := fmt.Sprintf("%s-%d-%s", containerID, hostID, thresholdType)

	ns.thresholdMu.Lock()
	defer ns.thresholdMu.Unlock()

	if tracker, exists := ns.thresholdState[key]; exists {
		tracker.LastNotifiedAt = time.Now()
	}
}

// detectAnomalies detects anomalous behavior after image updates
func (ns *NotificationService) detectAnomalies(hostID int64) ([]models.NotificationEvent, error) {
	var events []models.NotificationEvent

	// Get containers that have been updated recently (within last 1 hour)
	// and compare their current stats against baseline
	containers, err := ns.db.GetContainersByHost(hostID)
	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		if container.State != "running" || container.MemoryLimit == 0 {
			continue
		}

		// Get baseline stats for this container's previous image
		baseline, err := ns.db.GetContainerBaseline(container.ID, container.HostID)
		if err != nil || baseline == nil {
			continue // No baseline yet
		}

		// Check if current stats are anomalous (>25% increase)
		cpuIncrease := (container.CPUPercent - baseline.AvgCPUPercent) / baseline.AvgCPUPercent
		memoryIncrease := (container.MemoryPercent - baseline.AvgMemoryPercent) / baseline.AvgMemoryPercent

		if cpuIncrease > 0.25 || memoryIncrease > 0.25 {
			events = append(events, models.NotificationEvent{
				EventType:     models.EventTypeAnomalousBehavior,
				Timestamp:     time.Now(),
				ContainerID:   container.ID,
				ContainerName: container.Name,
				HostID:        container.HostID,
				HostName:      container.HostName,
				Image:         container.Image,
				CPUPercent:    container.CPUPercent,
				MemoryPercent: container.MemoryPercent,
				Metadata: map[string]interface{}{
					"baseline_cpu":      baseline.AvgCPUPercent,
					"baseline_memory":   baseline.AvgMemoryPercent,
					"cpu_increase_pct":  cpuIncrease * 100,
					"mem_increase_pct":  memoryIncrease * 100,
				},
			})
		}
	}

	return events, nil
}

// matchRules matches events against notification rules
func (ns *NotificationService) matchRules(ctx context.Context, events []models.NotificationEvent) ([]notificationTask, error) {
	var tasks []notificationTask

	// Get all enabled rules
	rules, err := ns.db.GetNotificationRules(true)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		for _, rule := range rules {
			if ns.ruleMatchesEvent(rule, event) {
				// Get channels for this rule
				channelIDs := rule.ChannelIDs
				for _, channelID := range channelIDs {
					// Check cooldown
					if ns.isInCooldown(rule.ID, event.ContainerID, event.HostID, rule.CooldownSeconds) {
						log.Printf("Skipping notification for rule %d (cooldown active)", rule.ID)
						continue
					}

					tasks = append(tasks, notificationTask{
						Rule:    rule,
						Event:   event,
						Channel: channelID,
					})
				}
			}
		}
	}

	return tasks, nil
}

// notificationTask represents a single notification to be sent
type notificationTask struct {
	Rule    models.NotificationRule
	Event   models.NotificationEvent
	Channel int64
}

// ruleMatchesEvent checks if a rule matches an event
func (ns *NotificationService) ruleMatchesEvent(rule models.NotificationRule, event models.NotificationEvent) bool {
	// Check event type
	eventTypeMatches := false
	for _, et := range rule.EventTypes {
		if et == event.EventType {
			eventTypeMatches = true
			break
		}
	}
	if !eventTypeMatches {
		return false
	}

	// Check host filter
	if rule.HostID != nil && *rule.HostID != event.HostID {
		return false
	}

	// Check container pattern
	if rule.ContainerPattern != "" {
		matched, err := filepath.Match(rule.ContainerPattern, event.ContainerName)
		if err != nil || !matched {
			return false
		}
	}

	// Check image pattern
	if rule.ImagePattern != "" {
		matched, err := filepath.Match(rule.ImagePattern, event.Image)
		if err != nil || !matched {
			return false
		}
	}

	// Check CPU threshold for high CPU events
	if event.EventType == models.EventTypeHighCPU && rule.CPUThreshold != nil {
		if event.CPUPercent < *rule.CPUThreshold {
			return false
		}
	}

	// Check memory threshold for high memory events
	if event.EventType == models.EventTypeHighMemory && rule.MemoryThreshold != nil {
		if event.MemoryPercent < *rule.MemoryThreshold {
			return false
		}
	}

	return true
}

// isInCooldown checks if a notification is in cooldown period
func (ns *NotificationService) isInCooldown(ruleID int64, containerID string, hostID int64, cooldownSeconds int) bool {
	if cooldownSeconds == 0 {
		return false
	}

	lastNotified, err := ns.db.GetLastNotificationTime(ruleID, containerID, hostID)
	if err != nil || lastNotified == nil {
		return false
	}

	cooldownDuration := time.Duration(cooldownSeconds) * time.Second
	return time.Since(*lastNotified) < cooldownDuration
}

// filterSilenced removes silenced notifications
func (ns *NotificationService) filterSilenced(tasks []notificationTask) []notificationTask {
	silences, err := ns.db.GetActiveSilences()
	if err != nil {
		log.Printf("Warning: Failed to get silences: %v", err)
		return tasks
	}

	var filtered []notificationTask
	for _, task := range tasks {
		silenced := false
		for _, silence := range silences {
			if ns.silenceMatches(silence, task.Event) {
				silenced = true
				log.Printf("Notification silenced: %s on %s (reason: %s)", task.Event.ContainerName, task.Event.HostName, silence.Reason)
				break
			}
		}
		if !silenced {
			filtered = append(filtered, task)
		}
	}

	return filtered
}

// silenceMatches checks if a silence applies to an event
func (ns *NotificationService) silenceMatches(silence models.NotificationSilence, event models.NotificationEvent) bool {
	// Check if silence is still active
	if time.Now().After(silence.SilencedUntil) {
		return false
	}

	// Check exact host match
	if silence.HostID != nil && *silence.HostID == event.HostID {
		// If container is also specified, check it
		if silence.ContainerID != "" && silence.ContainerID == event.ContainerID {
			return true
		}
		// If no container specified, silence applies to whole host
		if silence.ContainerID == "" {
			return true
		}
	}

	// Check exact container match
	if silence.ContainerID != "" && silence.ContainerID == event.ContainerID &&
	   silence.HostID != nil && *silence.HostID == event.HostID {
		return true
	}

	// Check patterns
	if silence.ContainerPattern != "" {
		matched, err := filepath.Match(silence.ContainerPattern, event.ContainerName)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// sendNotifications sends notifications with rate limiting
func (ns *NotificationService) sendNotifications(ctx context.Context, tasks []notificationTask) error {
	if len(tasks) == 0 {
		return nil
	}

	// Group tasks by channel for batching if rate limited
	for _, task := range tasks {
		// Check rate limit
		if !ns.rateLimiter.Allow() {
			// Rate limited - add to batch queue
			ns.rateLimiter.AddToBatch(task)
			log.Printf("Rate limited: Queuing notification for later")
			continue
		}

		// Send immediately
		go ns.sendSingleNotification(ctx, task)
	}

	return nil
}

// sendSingleNotification sends a single notification
func (ns *NotificationService) sendSingleNotification(ctx context.Context, task notificationTask) {
	// Get channel
	channel, err := ns.getChannel(task.Channel)
	if err != nil {
		log.Printf("Error getting channel %d: %v", task.Channel, err)
		ns.logNotification(task, false, fmt.Sprintf("Channel not found: %v", err))
		return
	}

	// Build message
	message := ns.buildMessage(task.Event)

	// Send notification
	err = channel.Send(ctx, message, task.Event)
	if err != nil {
		log.Printf("Error sending notification via channel %d: %v", task.Channel, err)
		ns.logNotification(task, false, err.Error())
		return
	}

	// Log success
	ns.logNotification(task, true, "")

	// Reset threshold if applicable
	if task.Event.EventType == models.EventTypeHighCPU {
		ns.ResetThreshold(task.Event.ContainerID, task.Event.HostID, "cpu")
	} else if task.Event.EventType == models.EventTypeHighMemory {
		ns.ResetThreshold(task.Event.ContainerID, task.Event.HostID, "memory")
	}
}

// buildMessage creates a human-readable message from an event
func (ns *NotificationService) buildMessage(event models.NotificationEvent) string {
	switch event.EventType {
	case models.EventTypeNewImage:
		return fmt.Sprintf("🔄 Image updated for %s on %s: %s → %s",
			event.ContainerName, event.HostName, event.OldImage, event.NewImage)
	case models.EventTypeContainerStarted:
		return fmt.Sprintf("✅ Container started: %s on %s", event.ContainerName, event.HostName)
	case models.EventTypeContainerStopped:
		return fmt.Sprintf("🛑 Container stopped: %s on %s", event.ContainerName, event.HostName)
	case models.EventTypeContainerPaused:
		return fmt.Sprintf("⏸️ Container paused: %s on %s", event.ContainerName, event.HostName)
	case models.EventTypeContainerResumed:
		return fmt.Sprintf("▶️ Container resumed: %s on %s", event.ContainerName, event.HostName)
	case models.EventTypeHighCPU:
		return fmt.Sprintf("⚠️ High CPU usage: %s on %s (%.1f%%)",
			event.ContainerName, event.HostName, event.CPUPercent)
	case models.EventTypeHighMemory:
		return fmt.Sprintf("⚠️ High memory usage: %s on %s (%.1f%%)",
			event.ContainerName, event.HostName, event.MemoryPercent)
	case models.EventTypeAnomalousBehavior:
		return fmt.Sprintf("🔍 Anomalous behavior detected: %s on %s (CPU: %.1f%%, Memory: %.1f%%)",
			event.ContainerName, event.HostName, event.CPUPercent, event.MemoryPercent)
	case models.EventTypeStateChange:
		return fmt.Sprintf("🔄 State changed: %s on %s (%s → %s)",
			event.ContainerName, event.HostName, event.OldState, event.NewState)
	default:
		return fmt.Sprintf("Event: %s for %s on %s", event.EventType, event.ContainerName, event.HostName)
	}
}

// logNotification logs a notification to the database
func (ns *NotificationService) logNotification(task notificationTask, success bool, errorMsg string) {
	metadataJSON, _ := json.Marshal(task.Event.Metadata)

	notifLog := models.NotificationLog{
		RuleID:        &task.Rule.ID,
		ChannelID:     &task.Channel,
		EventType:     task.Event.EventType,
		ContainerID:   task.Event.ContainerID,
		ContainerName: task.Event.ContainerName,
		HostID:        &task.Event.HostID,
		HostName:      task.Event.HostName,
		Message:       ns.buildMessage(task.Event),
		SentAt:        time.Now(),
		Success:       success,
		Error:         errorMsg,
		Read:          false,
	}

	if len(task.Event.Metadata) > 0 {
		var metadata map[string]interface{}
		json.Unmarshal(metadataJSON, &metadata)
		notifLog.Metadata = metadata
	}

	if err := ns.db.SaveNotificationLog(notifLog); err != nil {
		log.Printf("Failed to save notification log: %v", err)
	}
}

// getChannel retrieves a channel instance
func (ns *NotificationService) getChannel(channelID int64) (channels.Channel, error) {
	ns.channelsMu.RLock()
	ch, exists := ns.channels[channelID]
	ns.channelsMu.RUnlock()

	if exists {
		return ch, nil
	}

	// Load from database
	dbChannel, err := ns.db.GetNotificationChannel(channelID)
	if err != nil {
		return nil, err
	}

	if !dbChannel.Enabled {
		return nil, fmt.Errorf("channel %d is disabled", channelID)
	}

	// Create channel instance
	ch, err = ns.createChannelInstance(dbChannel)
	if err != nil {
		return nil, err
	}

	// Cache it
	ns.channelsMu.Lock()
	ns.channels[channelID] = ch
	ns.channelsMu.Unlock()

	return ch, nil
}

// createChannelInstance creates a channel instance from database config
func (ns *NotificationService) createChannelInstance(ch *models.NotificationChannel) (channels.Channel, error) {
	switch ch.Type {
	case models.ChannelTypeWebhook:
		return channels.NewWebhookChannel(ch)
	case models.ChannelTypeNtfy:
		return channels.NewNtfyChannel(ch)
	case models.ChannelTypeInApp:
		return channels.NewInAppChannel(ch, ns.db)
	default:
		return nil, fmt.Errorf("unknown channel type: %s", ch.Type)
	}
}

// RefreshChannels reloads all channels from database (called after config changes)
func (ns *NotificationService) RefreshChannels() {
	ns.channelsMu.Lock()
	defer ns.channelsMu.Unlock()

	// Clear cache to force reload
	ns.channels = make(map[int64]channels.Channel)
}

// SendTestNotification sends a test notification to a specific channel
func (ns *NotificationService) SendTestNotification(ctx context.Context, channelID int64) error {
	// Get the channel from database
	channel, err := ns.db.GetNotificationChannel(channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	// Create the channel instance
	ch, err := ns.createChannelInstance(channel)
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Send test notification using the channel's Test method
	if err := ch.Test(ctx); err != nil {
		return fmt.Errorf("failed to send test notification: %w", err)
	}

	log.Printf("Test notification sent successfully to channel: %s (%s)", channel.Name, channel.Type)
	return nil
}

// GetRateLimiter returns the rate limiter instance
func (ns *NotificationService) GetRateLimiter() *RateLimiter {
	return ns.rateLimiter
}
