package channels

import (
	"context"

	"github.com/container-census/container-census/internal/models"
)

// Channel represents a notification delivery channel
type Channel interface {
	// Send sends a notification
	Send(ctx context.Context, message string, event models.NotificationEvent) error

	// Test sends a test notification
	Test(ctx context.Context) error

	// Type returns the channel type
	Type() string

	// Name returns the channel name
	Name() string
}
