package storage

import (
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestCleanupOldNotifications tests clearing old notifications
func TestCleanupOldNotifications(t *testing.T) {
	// Create temporary database
	tmpfile, err := os.CreateTemp("", "clear-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	db, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}

	// Create a host
	host := models.Host{Name: "clear-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create old logs (8 days old) - should be deleted
	for i := 0; i < 5; i++ {
		log := models.NotificationLog{
			RuleName:      "old-rule",
			EventType:     "container_stopped",
			ContainerName: "old-container",
			HostID:        &hostID,
			Message:       "Old notification",
			SentAt:        now.Add(-8 * 24 * time.Hour),
			Read:          true,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save old log: %v", err)
		}
	}

	// Create recent logs - should be kept
	for i := 0; i < 3; i++ {
		log := models.NotificationLog{
			RuleName:      "new-rule",
			EventType:     "new_image",
			ContainerName: "new-container",
			HostID:        &hostID,
			Message:       "Recent notification",
			SentAt:        now.Add(-1 * time.Hour),
			Read:          false,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save recent log: %v", err)
		}
	}

	// Get initial count
	beforeLogs, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	beforeCount := len(beforeLogs)
	t.Logf("Logs before cleanup: %d", beforeCount)

	if beforeCount != 8 {
		t.Fatalf("Expected 8 logs before cleanup, got %d", beforeCount)
	}

	// Clear old logs
	err = db.CleanupOldNotifications()
	if err != nil {
		t.Fatalf("CleanupOldNotifications failed: %v", err)
	}

	// Get logs after clear
	afterLogs, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	afterCount := len(afterLogs)

	t.Logf("Logs after cleanup: %d", afterCount)

	// Should have removed old logs but kept recent ones
	// Implementation should keep 100 most recent OR delete those older than 7 days
	if afterCount >= beforeCount {
		t.Errorf("Expected logs to be cleaned up, before: %d, after: %d", beforeCount, afterCount)
	}

	// Should keep the 3 recent logs
	if afterCount != 3 {
		t.Errorf("Expected 3 recent logs to remain, got %d", afterCount)
	}

	// Verify recent logs are still there
	foundRecent := false
	for _, log := range afterLogs {
		if log.RuleName == "new-rule" {
			foundRecent = true
			break
		}
	}

	if !foundRecent {
		t.Error("Recent logs should be preserved")
	}

	// Verify old logs are gone
	foundOld := false
	for _, log := range afterLogs {
		if log.RuleName == "old-rule" {
			foundOld = true
			break
		}
	}

	if foundOld {
		t.Error("Old logs (8+ days) should be deleted")
	}

	t.Log("✓ CleanupOldNotifications working correctly!")
}
