package storage

import (
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestCleanupSimple is a minimal test for notification cleanup
func TestCleanupSimple(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cleanup-simple-*.db")
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
	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}

	// Create 5 old notifications (10 days old)
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	for i := 0; i < 5; i++ {
		log := models.NotificationLog{
			RuleName:      "old",
			EventType:     "test",
			ContainerName: "old",
			HostID:        &hostID,
			Message:       "Old",
			SentAt:        oldTime,
			Success:       true,
			Read:          true,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save old log: %v", err)
		}
	}

	// Create 3 recent notifications (1 hour old)
	recentTime := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 3; i++ {
		log := models.NotificationLog{
			RuleName:      "recent",
			EventType:     "test",
			ContainerName: "recent",
			HostID:        &hostID,
			Message:       "Recent",
			SentAt:        recentTime,
			Success:       true,
			Read:          false,
		}
		if err := db.SaveNotificationLog(log); err != nil {
			t.Fatalf("Failed to save recent log: %v", err)
		}
	}

	// Verify we have 8 notifications
	before, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	t.Logf("Before cleanup: %d notifications", len(before))
	if len(before) != 8 {
		t.Fatalf("Expected 8 notifications before cleanup, got %d", len(before))
	}

	// Run cleanup
	if err := db.CleanupOldNotifications(); err != nil {
		t.Fatalf("CleanupOldNotifications failed: %v", err)
	}

	// Check after cleanup
	after, err := db.GetNotificationLogs(1000, false)
	if err != nil {
		t.Fatalf("GetNotificationLogs failed: %v", err)
	}
	t.Logf("After cleanup: %d notifications", len(after))

	// Should have deleted the 5 old ones, keeping 3 recent
	if len(after) != 3 {
		t.Errorf("Expected 3 notifications after cleanup, got %d", len(after))
		for _, log := range after {
			t.Logf("  Remaining: %s at %v", log.RuleName, log.SentAt)
		}
	}

	// Verify the remaining ones are the recent ones by checking timestamps
	recentCount := 0
	for _, log := range after {
		if log.SentAt.After(time.Now().Add(-2 * time.Hour)) {
			recentCount++
		}
	}

	if recentCount != 3 {
		t.Errorf("Expected 3 recent logs (within 2 hours), got %d", recentCount)
	}

	t.Log("✅ Cleanup working correctly!")
}
