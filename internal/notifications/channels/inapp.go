package channels

import (
	"context"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

// InAppChannel implements in-app notifications (writes to notification_log)
type InAppChannel struct {
	name string
	db   *storage.DB
}

// NewInAppChannel creates a new in-app channel
func NewInAppChannel(ch *models.NotificationChannel, db *storage.DB) (*InAppChannel, error) {
	return &InAppChannel{
		name: ch.Name,
		db:   db,
	}, nil
}

// Send sends an in-app notification (writes to database)
func (iac *InAppChannel) Send(ctx context.Context, message string, event models.NotificationEvent) error {
	// In-app notifications are already logged by the notifier
	// This is just a no-op implementation since logging happens at a higher level
	return nil
}

// Test sends a test notification
func (iac *InAppChannel) Test(ctx context.Context) error {
	testLog := models.NotificationLog{
		EventType:     "test",
		ContainerName: "test-container",
		HostName:      "test-host",
		Message:       "🧪 Test notification from Container Census",
		SentAt:        time.Now(),
		Success:       true,
		Read:          false,
	}

	if err := iac.db.SaveNotificationLog(testLog); err != nil {
		return err
	}

	// Log for debugging
	println("✅ In-app test notification saved to database")
	return nil
}

// Type returns the channel type
func (iac *InAppChannel) Type() string {
	return models.ChannelTypeInApp
}

// Name returns the channel name
func (iac *InAppChannel) Name() string {
	return iac.name
}
