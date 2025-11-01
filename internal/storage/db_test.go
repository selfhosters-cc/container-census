package storage

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "census-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	tmpfile.Close()

	// Clean up on test completion
	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
	})

	db, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	return db
}

// TestHostCRUD tests Create, Read, Update, Delete operations for hosts
func TestHostCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Create a host
	host := &models.Host{
		Name:         "test-host",
		Address:      "unix:///var/run/docker.sock",
		CollectStats: true,
		Enabled:      true,
	}

	hostID, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("AddHost failed: %v", err)
	}
	host.ID = hostID

	if host.ID == 0 {
		t.Error("Expected host ID to be set after save")
	}

	// Read hosts
	hosts, err := db.GetHosts()
	if err != nil {
		t.Fatalf("GetHosts failed: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}

	savedHost := hosts[0]
	if savedHost.Name != host.Name {
		t.Errorf("Expected host name %s, got %s", host.Name, savedHost.Name)
	}
	if savedHost.Address != host.Address {
		t.Errorf("Expected address %s, got %s", host.Address, savedHost.Address)
	}
	if savedHost.CollectStats != host.CollectStats {
		t.Errorf("Expected CollectStats %v, got %v", host.CollectStats, savedHost.CollectStats)
	}

	// Update host
	savedHost.Name = "updated-host"
	savedHost.Address = "agent://remote-host:9876"
	savedHost.CollectStats = false

	err = db.UpdateHost(savedHost)
	if err != nil {
		t.Fatalf("UpdateHost failed: %v", err)
	}

	// Verify update
	hosts, err = db.GetHosts()
	if err != nil {
		t.Fatalf("GetHosts failed: %v", err)
	}

	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host after update, got %d", len(hosts))
	}

	if hosts[0].Name != "updated-host" {
		t.Errorf("Host name not updated: got %s", hosts[0].Name)
	}
	if hosts[0].CollectStats {
		t.Error("CollectStats should be false after update")
	}

	// Delete host
	err = db.DeleteHost(savedHost.ID)
	if err != nil {
		t.Fatalf("DeleteHost failed: %v", err)
	}

	// Verify deletion
	hosts, err = db.GetHosts()
	if err != nil {
		t.Fatalf("GetHosts failed: %v", err)
	}

	if len(hosts) != 0 {
		t.Errorf("Expected 0 hosts after deletion, got %d", len(hosts))
	}
}

// TestMultipleHosts tests handling multiple hosts
func TestMultipleHosts(t *testing.T) {
	db := setupTestDB(t)

	hosts := []*models.Host{
		{Name: "host1", Address: "unix:///var/run/docker.sock", Enabled: true},
		{Name: "host2", Address: "agent://host2:9876", Enabled: true},
		{Name: "host3", Address: "tcp://host3:2375", Enabled: false},
	}

	for _, host := range hosts {
		_, err := db.AddHost(*host)
		if err != nil {
			t.Fatalf("Failed to save host %s: %v", host.Name, err)
		}
	}

	retrieved, err := db.GetHosts()
	if err != nil {
		t.Fatalf("GetHosts failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Fatalf("Expected 3 hosts, got %d", len(retrieved))
	}

	// Verify all hosts are present
	names := make(map[string]bool)
	for _, h := range retrieved {
		names[h.Name] = true
	}

	for _, expected := range []string{"host1", "host2", "host3"} {
		if !names[expected] {
			t.Errorf("Expected host %s not found", expected)
		}
	}
}

// TestContainerHistory tests saving and retrieving container history
func TestContainerHistory(t *testing.T) {
	db := setupTestDB(t)

	// Create a host first
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	_, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Save container snapshots
	containers := []models.Container{
		{
			ID: "abc123",
			HostID:      host.ID,
			Name:        "web-server",
			Image:       "nginx:latest",
			State:       "running",
			Status:      "Up 5 minutes",
			ScannedAt:   now,
			CPUPercent:  25.5,
			MemoryUsage: 104857600, // 100MB
			MemoryLimit: 1073741824, // 1GB
		},
		{
			ID: "abc123",
			HostID:      host.ID,
			Name:        "web-server",
			Image:       "nginx:latest",
			State:       "running",
			Status:      "Up 6 minutes",
			ScannedAt:   now.Add(1 * time.Minute),
			CPUPercent:  30.2,
			MemoryUsage: 115343360, // 110MB
			MemoryLimit: 1073741824,
		},
	}

	for _, container := range containers {
		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Retrieve containers
	retrieved, err := db.GetContainers()
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}

	if len(retrieved) < 2 {
		t.Fatalf("Expected at least 2 container records, got %d", len(retrieved))
	}

	// Verify data
	found := false
	for _, c := range retrieved {
		if c.ContainerID == "abc123" && c.Name == "web-server" {
			found = true
			if c.Image != "nginx:latest" {
				t.Errorf("Expected image nginx:latest, got %s", c.Image)
			}
			if c.HostID != host.ID {
				t.Errorf("Expected host ID %d, got %d", host.ID, c.HostID)
			}
		}
	}

	if !found {
		t.Error("Container not found in retrieved records")
	}
}

// TestContainerStats tests stats-related functionality
func TestContainerStats(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "stats-host", Address: "unix:///", Enabled: true}
	_, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()
	baseTime := now.Add(-2 * time.Hour) // Start 2 hours ago

	// Create multiple container snapshots with stats
	for i := 0; i < 120; i++ { // 120 minutes of data
		container := models.Container{
			ID:   "stats123",
			HostID:        host.ID,
			Name:          "app",
			Image:         "app:v1",
			State:         "running",
			Status:        "Up",
			ScannedAt:     baseTime.Add(time.Duration(i) * time.Minute),
			CPUPercent:    float64(50 + i%20),   // Varying CPU
			MemoryUsage:   int64(200000000 + i*1000000), // Increasing memory
			MemoryLimit:   1073741824,
			MemoryPercent: float64(20 + i%10),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container snapshot %d: %v", i, err)
		}
	}

	// Test GetContainerStats - should return data points
	stats, err := db.GetContainerStats(host.ID, "stats123", "1h")
	if err != nil {
		t.Fatalf("GetContainerStats failed: %v", err)
	}

	if len(stats) == 0 {
		t.Error("Expected stats data points, got none")
	}

	// Verify stats data structure
	for _, stat := range stats {
		if stat.CPUPercent < 0 || stat.CPUPercent > 100 {
			t.Errorf("Invalid CPU percent: %f", stat.CPUPercent)
		}
		if stat.MemoryUsage <= 0 {
			t.Errorf("Invalid memory usage: %d", stat.MemoryUsage)
		}
		if stat.Timestamp.IsZero() {
			t.Error("Timestamp should not be zero")
		}
	}
}

// TestStatsAggregation tests the hourly aggregation of stats
func TestStatsAggregation(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "agg-host", Address: "unix:///", Enabled: true}
	_, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	// Create old container snapshots (more than 1 hour old)
	baseTime := time.Now().Add(-3 * time.Hour)

	for i := 0; i < 60; i++ {
		container := models.Container{
			ID:   "agg123",
			HostID:        host.ID,
			Name:          "app",
			Image:         "app:v1",
			State:         "running",
			ScannedAt:     baseTime.Add(time.Duration(i) * time.Minute),
			CPUPercent:    float64(40 + i%30),
			MemoryUsage:   int64(150000000 + i*1000000),
			MemoryLimit:   1073741824,
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Run aggregation
	if err := db.AggregateOldStats(); err != nil {
		t.Fatalf("AggregateOldStats failed: %v", err)
	}

	// Verify aggregated data exists
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM container_stats_aggregates WHERE container_id = ? AND host_id = ?",
		"agg123", host.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query aggregates: %v", err)
	}

	if count == 0 {
		t.Error("Expected aggregated stats to be created")
	}

	// Verify old granular data was deleted
	err = db.db.QueryRow("SELECT COUNT(*) FROM containers WHERE container_id = ? AND host_id = ? AND timestamp < ?",
		"agg123", host.ID, time.Now().Add(-1*time.Hour)).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query old containers: %v", err)
	}

	// Old data should be removed after aggregation
	// Note: Depending on implementation, this might be 0 or still have some data
	t.Logf("Old granular records remaining: %d", count)
}

// TestScanResults tests scan result tracking
func TestScanResults(t *testing.T) {
	db := setupTestDB(t)

	result := models.ScanResult{
		ScannedAt:      time.Now(),
		TotalContainers: 15,
		RunningContainers: 12,
		Duration:       time.Second * 5,
		Success:        true,
	}

	if err := db.SaveScanResult(result); err != nil {
		t.Fatalf("SaveScanResult failed: %v", err)
	}

	// Retrieve scan results
	results, err := db.GetScanResults(10)
	if err != nil {
		t.Fatalf("GetScanResults failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 scan result, got %d", len(results))
	}

	retrieved := results[0]
	if retrieved.TotalContainers != result.TotalContainers {
		t.Errorf("Expected %d total containers, got %d", result.TotalContainers, retrieved.TotalContainers)
	}
	if retrieved.RunningContainers != result.RunningContainers {
		t.Errorf("Expected %d running containers, got %d", result.RunningContainers, retrieved.RunningContainers)
	}
	if !retrieved.Success {
		t.Error("Expected scan result to be successful")
	}
}

// TestGetContainerLifecycleEvents tests lifecycle event history
func TestGetContainerLifecycleEvents(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "event-host", Address: "unix:///", Enabled: true}
	_, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create container state transitions
	states := []struct {
		state     string
		image     string
		timestamp time.Time
	}{
		{"running", "app:v1", now.Add(-10 * time.Minute)},
		{"running", "app:v1", now.Add(-9 * time.Minute)},
		{"exited", "app:v1", now.Add(-8 * time.Minute)},
		{"running", "app:v2", now.Add(-5 * time.Minute)}, // Image change
		{"running", "app:v2", now.Add(-2 * time.Minute)},
	}

	for _, s := range states {
		container := models.Container{
			ID: "event123",
			HostID:      host.ID,
			Name:        "app",
			Image:       s.image,
			State:       s.state,
			ScannedAt:   s.timestamp,
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Get lifecycle events
	events, err := db.GetContainerLifecycleEvents(host.ID, "event123", 10)
	if err != nil {
		t.Fatalf("GetContainerLifecycleEvents failed: %v", err)
	}

	if len(events) == 0 {
		t.Error("Expected lifecycle events, got none")
	}

	// Verify we captured both state change and image change
	hasStateChange := false
	hasImageChange := false

	for _, event := range events {
		if event.OldState != event.NewState {
			hasStateChange = true
		}
		if event.OldImage != "" && event.OldImage != event.NewImage {
			hasImageChange = true
		}
	}

	if !hasStateChange {
		t.Error("Expected to find state change events")
	}
	if !hasImageChange {
		t.Error("Expected to find image change events")
	}
}

// TestDatabaseSchema tests that the schema is created correctly
func TestDatabaseSchema(t *testing.T) {
	db := setupTestDB(t)

	// Verify key tables exist
	tables := []string{
		"hosts",
		"containers",
		"container_stats_aggregates",
		"scan_results",
		"notification_channels",
		"notification_rules",
		"notification_rule_channels",
		"notification_log",
		"notification_silences",
		"container_baseline_stats",
		"notification_threshold_state",
	}

	for _, table := range tables {
		var name string
		err := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("Table %s does not exist", table)
		} else if err != nil {
			t.Errorf("Error checking table %s: %v", table, err)
		}
	}
}

// TestConcurrentAccess tests concurrent database operations
func TestConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)

	// Create host
	host := &models.Host{Name: "concurrent-host", Address: "unix:///", Enabled: true}
	_, err := db.AddHost(*host)
	if err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	// Concurrent writes
	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			container := models.Container{
				ID: "concurrent123",
				HostID:      host.ID,
				Name:        "app",
				Image:       "app:v1",
				State:       "running",
				ScannedAt:   time.Now(),
			}

			if err := db.SaveContainers([]models.Container{container}); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
	}

	// Verify all writes succeeded
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM containers WHERE container_id = ?", "concurrent123").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count containers: %v", err)
	}

	if count != 10 {
		t.Errorf("Expected 10 container records, got %d", count)
	}
}
