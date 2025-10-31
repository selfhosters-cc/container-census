package storage

import (
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// TestSQLDatetimeDebug directly tests SQL datetime logic
func TestSQLDatetimeDebug(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "sql-debug-*.db")
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
	host := models.Host{Name: "test", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}

	// Add one old record
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	log := models.NotificationLog{
		RuleName:      "old",
		EventType:     "test",
		ContainerName: "test",
		HostID:        &hostID,
		Message:       "Old",
		SentAt:        oldTime,
		Success:       true,
		Read:          false,
	}
	if err := db.SaveNotificationLog(log); err != nil {
		t.Fatalf("Failed to save log: %v", err)
	}

	// Direct SQL queries to debug
	conn := db.conn

	// Query 1: What's datetime('now', '-7 days')?
	var sevenDaysAgo string
	err = conn.QueryRow("SELECT datetime('now', '-7 days')").Scan(&sevenDaysAgo)
	if err != nil {
		t.Fatalf("Query 1 failed: %v", err)
	}
	t.Logf("datetime('now', '-7 days') = %s", sevenDaysAgo)

	// Query 2: What's stored in sent_at?
	var storedTime string
	err = conn.QueryRow("SELECT sent_at FROM notification_log LIMIT 1").Scan(&storedTime)
	if err != nil {
		t.Fatalf("Query 2 failed: %v", err)
	}
	t.Logf("stored sent_at = %s", storedTime)

	// Query 3: Direct comparison
	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM notification_log WHERE sent_at < datetime('now', '-7 days')").Scan(&count)
	if err != nil {
		t.Fatalf("Query 3 failed: %v", err)
	}
	t.Logf("Records where sent_at < datetime('now', '-7 days'): %d", count)

	if count == 0 {
		t.Error("❌ No records matched the datetime comparison - this is the bug!")

		// Try different comparison approaches
		var count2 int
		err = conn.QueryRow("SELECT COUNT(*) FROM notification_log WHERE datetime(sent_at) < datetime('now', '-7 days')").Scan(&count2)
		if err == nil {
			t.Logf("With datetime() wrapper: %d matches", count2)
		}

		var count3 int
		err = conn.QueryRow("SELECT COUNT(*) FROM notification_log WHERE julianday(sent_at) < julianday('now', '-7 days')").Scan(&count3)
		if err == nil {
			t.Logf("With julianday: %d matches", count3)
		}
	} else {
		t.Logf("✅ Records matched! The SQL comparison works.")
	}
}
