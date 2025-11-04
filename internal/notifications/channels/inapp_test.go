package channels

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

// setupTestInAppChannel creates a test in-app channel with database
func setupTestInAppChannel(t *testing.T) (*InAppChannel, *storage.DB) {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "inapp-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	tmpfile.Close()

	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
	})

	db, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	channel := &models.NotificationChannel{
		Name:   "in-app",
		Type:   "inapp",
		Config: map[string]interface{}{},
	}

	iac, err := NewInAppChannel(channel, db)
	if err != nil {
		t.Fatalf("NewInAppChannel failed: %v", err)
	}

	return iac, db
}

// TestInAppChannel_BasicSend tests basic in-app notification
func TestInAppChannel_BasicSend(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	// Create host for the event
	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	event := models.NotificationEvent{
		EventType:     "container_stopped",
		ContainerID:   "test123",
		ContainerName: "web-server",
		HostID:        host.ID,
		HostName:      "test-host",
		Image:         "nginx:latest",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = iac.Send(ctx, "Container stopped", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify notification was logged
	logs, err := db.GetNotificationLogs(10, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 notification log, got %d", len(logs))
	}

	log := logs[0]
	if log.Message != "Container stopped" {
		t.Errorf("Expected message 'Container stopped', got '%s'", log.Message)
	}

	if log.ContainerName != "web-server" {
		t.Errorf("Expected container name 'web-server', got '%s'", log.ContainerName)
	}

	if log.EventType != "container_stopped" {
		t.Errorf("Expected event type 'container_stopped', got '%s'", log.EventType)
	}

	if log.Read {
		t.Error("New notification should be unread")
	}
}

// TestInAppChannel_AllEventTypes tests different event types
func TestInAppChannel_AllEventTypes(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	events := []struct {
		eventType string
		message   string
	}{
		{"container_stopped", "Container stopped"},
		{"container_started", "Container started"},
		{"new_image", "New image detected"},
		{"high_cpu", "High CPU usage"},
		{"high_memory", "High memory usage"},
		{"anomalous_behavior", "Anomalous behavior detected"},
	}

	ctx := context.Background()

	for _, e := range events {
		event := models.NotificationEvent{
			EventType:     e.eventType,
			ContainerID:   "test123",
			ContainerName: "test-container",
			HostID:        host.ID,
			Timestamp:     time.Now(),
		}

		err := iac.Send(ctx, e.message, event)
		if err != nil {
			t.Errorf("Send failed for %s: %v", e.eventType, err)
		}
	}

	// Verify all notifications were logged
	logs, err := db.GetNotificationLogs(100, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != len(events) {
		t.Errorf("Expected %d notifications, got %d", len(events), len(logs))
	}
}

// TestInAppChannel_WithMetadata tests notification with event metadata
func TestInAppChannel_WithMetadata(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	event := models.NotificationEvent{
		EventType:     "high_cpu",
		ContainerID:   "test123",
		ContainerName: "cpu-hog",
		HostID:        host.ID,
		CPUPercent:    85.5,
		MemoryPercent: 60.2,
		OldImage:      "app:v1",
		NewImage:      "app:v2",
		Timestamp:     time.Now(),
	}

	ctx := context.Background()
	err = iac.Send(ctx, "High CPU detected", event)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	logs, err := db.GetNotificationLogs(10, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(logs))
	}

	// Verify metadata fields are preserved in metadata map
	log := logs[0]
	if cpuVal, ok := log.Metadata["cpu_percent"].(float64); !ok || cpuVal != 85.5 {
		t.Errorf("Expected CPU 85.5 in metadata, got %v", log.Metadata["cpu_percent"])
	}

	if memVal, ok := log.Metadata["memory_percent"].(float64); !ok || memVal != 60.2 {
		t.Errorf("Expected memory 60.2 in metadata, got %v", log.Metadata["memory_percent"])
	}
}

// TestInAppChannel_Test tests the test notification
func TestInAppChannel_Test(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	ctx := context.Background()
	err := iac.Test(ctx)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// Verify test notification was logged
	logs, err := db.GetNotificationLogs(10, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != 1 {
		t.Fatalf("Expected 1 test notification, got %d", len(logs))
	}

	log := logs[0]
	if log.EventType != "test" {
		t.Errorf("Expected event type 'test', got '%s'", log.EventType)
	}
}

// TestInAppChannel_TypeAndName tests Type and Name methods
func TestInAppChannel_TypeAndName(t *testing.T) {
	iac, _ := setupTestInAppChannel(t)

	if iac.Type() != "inapp" {
		t.Errorf("Expected type 'inapp', got '%s'", iac.Type())
	}

	if iac.Name() != "in-app" {
		t.Errorf("Expected name 'in-app', got '%s'", iac.Name())
	}
}

// TestInAppChannel_MultipleNotifications tests sending multiple notifications
func TestInAppChannel_MultipleNotifications(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	ctx := context.Background()

	// Send multiple notifications
	for i := 0; i < 10; i++ {
		event := models.NotificationEvent{
			EventType:     "container_stopped",
			ContainerID:   "test123",
			ContainerName: "web-server",
			HostID:        host.ID,
			Timestamp:     time.Now(),
		}

		err := iac.Send(ctx, "Container stopped", event)
		if err != nil {
			t.Fatalf("Send #%d failed: %v", i+1, err)
		}
	}

	// Verify all were logged
	logs, err := db.GetNotificationLogs(100, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != 10 {
		t.Errorf("Expected 10 notifications, got %d", len(logs))
	}

	// All should be unread
	for i, log := range logs {
		if log.Read {
			t.Errorf("Notification #%d should be unread", i)
		}
	}
}

// TestInAppChannel_ConcurrentSends tests thread-safe sending
func TestInAppChannel_ConcurrentSends(t *testing.T) {
	iac, db := setupTestInAppChannel(t)

	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	ctx := context.Background()
	done := make(chan bool)
	errors := make(chan error, 10)

	// Concurrent sends
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := models.NotificationEvent{
				EventType:     "test",
				ContainerID:   "test123",
				ContainerName: "test",
				HostID:        host.ID,
				Timestamp:     time.Now(),
			}

			err := iac.Send(ctx, "Test notification", event)
			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent send error: %v", err)
	}

	// Verify all notifications were logged
	logs, err := db.GetNotificationLogs(100, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}

	if len(logs) != 10 {
		t.Errorf("Expected 10 notifications from concurrent sends, got %d", len(logs))
	}
}
