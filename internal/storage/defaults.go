package storage

import (
	"log"

	"github.com/container-census/container-census/internal/models"
)

// InitializeDefaultNotifications creates default notification channels and rules if they don't exist
func (db *DB) InitializeDefaultNotifications() error {
	// Check if any channels exist
	channels, err := db.GetNotificationChannels()
	if err != nil {
		return err
	}

	// If channels already exist, skip initialization
	if len(channels) > 0 {
		return nil
	}

	log.Println("Initializing default notification channels and rules...")

	// Create default in-app channel
	inAppChannel := &models.NotificationChannel{
		Name:    "In-App Notifications",
		Type:    models.ChannelTypeInApp,
		Config:  map[string]interface{}{},
		Enabled: true,
	}

	if err := db.SaveNotificationChannel(inAppChannel); err != nil {
		return err
	}

	log.Printf("Created default in-app notification channel (ID: %d)", inAppChannel.ID)

	// Create default rules
	rules := []models.NotificationRule{
		{
			Name:                     "Container Stopped",
			Enabled:                  true,
			EventTypes:               []string{models.EventTypeContainerStopped},
			ThresholdDurationSeconds: 120,
			CooldownSeconds:          300,
			ChannelIDs:               []int64{inAppChannel.ID},
		},
		{
			Name:                     "New Image Detected",
			Enabled:                  true,
			EventTypes:               []string{models.EventTypeNewImage},
			ThresholdDurationSeconds: 120,
			CooldownSeconds:          600,
			ChannelIDs:               []int64{inAppChannel.ID},
		},
		{
			Name:                     "High Resource Usage",
			Enabled:                  true,
			EventTypes:               []string{models.EventTypeHighCPU, models.EventTypeHighMemory},
			CPUThreshold:             floatPtr(80.0),
			MemoryThreshold:          floatPtr(90.0),
			ThresholdDurationSeconds: 120,
			CooldownSeconds:          600,
			ChannelIDs:               []int64{inAppChannel.ID},
		},
	}

	for _, rule := range rules {
		if err := db.SaveNotificationRule(&rule); err != nil {
			log.Printf("Warning: Failed to create default rule '%s': %v", rule.Name, err)
		} else {
			log.Printf("Created default notification rule: %s", rule.Name)
		}
	}

	return nil
}

// floatPtr returns a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}
