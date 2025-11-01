package storage

import (
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

func TestGetChangesReport(t *testing.T) {
	// Create a temporary database
	dbPath := "/tmp/test_reports.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Setup test data
	setupReportTestData(t, db)

	// Test cases
	tests := []struct {
		name       string
		start      time.Time
		end        time.Time
		hostFilter int64
		wantError  bool
		validate   func(t *testing.T, report *models.ChangesReport)
	}{
		{
			name:       "Last 7 days - no filter",
			start:      time.Now().Add(-7 * 24 * time.Hour),
			end:        time.Now(),
			hostFilter: 0,
			wantError:  false,
			validate: func(t *testing.T, report *models.ChangesReport) {
				if report == nil {
					t.Fatal("Expected non-nil report")
				}
				if report.Period.DurationHours != 168 {
					t.Errorf("Expected 168 hours, got %d", report.Period.DurationHours)
				}
			},
		},
		{
			name:       "Last 30 days - no filter",
			start:      time.Now().Add(-30 * 24 * time.Hour),
			end:        time.Now(),
			hostFilter: 0,
			wantError:  false,
			validate: func(t *testing.T, report *models.ChangesReport) {
				if report == nil {
					t.Fatal("Expected non-nil report")
				}
				if report.Summary.TotalHosts < 0 {
					t.Errorf("Expected non-negative host count, got %d", report.Summary.TotalHosts)
				}
			},
		},
		{
			name:       "With host filter",
			start:      time.Now().Add(-7 * 24 * time.Hour),
			end:        time.Now(),
			hostFilter: 1,
			wantError:  false,
			validate: func(t *testing.T, report *models.ChangesReport) {
				if report == nil {
					t.Fatal("Expected non-nil report")
				}
				// All containers should be from host 1
				for _, c := range report.NewContainers {
					if c.HostID != 1 {
						t.Errorf("Expected host_id 1, got %d", c.HostID)
					}
				}
				for _, c := range report.RemovedContainers {
					if c.HostID != 1 {
						t.Errorf("Expected host_id 1, got %d", c.HostID)
					}
				}
			},
		},
		{
			name:       "Empty time range",
			start:      time.Now().Add(1 * time.Hour),
			end:        time.Now().Add(2 * time.Hour),
			hostFilter: 0,
			wantError:  false,
			validate: func(t *testing.T, report *models.ChangesReport) {
				if report == nil {
					t.Fatal("Expected non-nil report")
				}
				// Should have zero changes
				if report.Summary.NewContainers != 0 {
					t.Errorf("Expected 0 new containers, got %d", report.Summary.NewContainers)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := db.GetChangesReport(tt.start, tt.end, tt.hostFilter)

			if (err != nil) != tt.wantError {
				t.Errorf("GetChangesReport() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, report)
			}
		})
	}
}

func TestGetChangesReport_NewContainers(t *testing.T) {
	dbPath := "/tmp/test_reports_new.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create host
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'test-host', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	// Insert a container that appeared 3 days ago
	threeDaysAgo := time.Now().Add(-3 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "abc123", "new-container", "nginx:latest", "sha256:abc123", "running", "Up 1 hour", threeDaysAgo, 1, "test-host", threeDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert container: %v", err)
	}

	// Test: Should find the new container in a 7-day window
	start := time.Now().Add(-7 * 24 * time.Hour)
	end := time.Now()
	report, err := db.GetChangesReport(start, end, 0)
	if err != nil {
		t.Fatalf("GetChangesReport failed: %v", err)
	}

	if len(report.NewContainers) != 1 {
		t.Errorf("Expected 1 new container, got %d", len(report.NewContainers))
	}

	if len(report.NewContainers) > 0 {
		c := report.NewContainers[0]
		if c.ContainerName != "new-container" {
			t.Errorf("Expected container name 'new-container', got '%s'", c.ContainerName)
		}
		if c.Image != "nginx:latest" {
			t.Errorf("Expected image 'nginx:latest', got '%s'", c.Image)
		}
		if c.State != "running" {
			t.Errorf("Expected state 'running', got '%s'", c.State)
		}
	}
}

func TestGetChangesReport_RemovedContainers(t *testing.T) {
	dbPath := "/tmp/test_reports_removed.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create host
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'test-host', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	// Insert a container that was last seen 10 days ago
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "old123", "removed-container", "redis:6", "sha256:old123", "exited", "Exited (0)", tenDaysAgo, 1, "test-host", tenDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert container: %v", err)
	}

	// Test: Should find the removed container (last seen before 7-day window)
	start := time.Now().Add(-7 * 24 * time.Hour)
	end := time.Now()
	report, err := db.GetChangesReport(start, end, 0)
	if err != nil {
		t.Fatalf("GetChangesReport failed: %v", err)
	}

	if len(report.RemovedContainers) != 1 {
		t.Errorf("Expected 1 removed container, got %d", len(report.RemovedContainers))
	}

	if len(report.RemovedContainers) > 0 {
		c := report.RemovedContainers[0]
		if c.ContainerName != "removed-container" {
			t.Errorf("Expected container name 'removed-container', got '%s'", c.ContainerName)
		}
	}
}

func TestGetChangesReport_ImageUpdates(t *testing.T) {
	dbPath := "/tmp/test_reports_images.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create host
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'test-host', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	// Insert container with old image (5 days ago)
	fiveDaysAgo := time.Now().Add(-5 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "web123", "web-app", "nginx:1.24", "sha256:old", "running", "Up", fiveDaysAgo, 1, "test-host", fiveDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert old container: %v", err)
	}

	// Insert same container with new image (2 days ago)
	twoDaysAgo := time.Now().Add(-2 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "web123", "web-app", "nginx:1.25", "sha256:new", "running", "Up", twoDaysAgo, 1, "test-host", twoDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert updated container: %v", err)
	}

	// Test: Should detect image update
	start := time.Now().Add(-7 * 24 * time.Hour)
	end := time.Now()
	report, err := db.GetChangesReport(start, end, 0)
	if err != nil {
		t.Fatalf("GetChangesReport failed: %v", err)
	}

	if len(report.ImageUpdates) != 1 {
		t.Errorf("Expected 1 image update, got %d", len(report.ImageUpdates))
	}

	if len(report.ImageUpdates) > 0 {
		u := report.ImageUpdates[0]
		if u.ContainerName != "web-app" {
			t.Errorf("Expected container name 'web-app', got '%s'", u.ContainerName)
		}
		if u.OldImage != "nginx:1.24" {
			t.Errorf("Expected old image 'nginx:1.24', got '%s'", u.OldImage)
		}
		if u.NewImage != "nginx:1.25" {
			t.Errorf("Expected new image 'nginx:1.25', got '%s'", u.NewImage)
		}
		if u.OldImageID != "sha256:old" {
			t.Errorf("Expected old image ID 'sha256:old', got '%s'", u.OldImageID)
		}
		if u.NewImageID != "sha256:new" {
			t.Errorf("Expected new image ID 'sha256:new', got '%s'", u.NewImageID)
		}
	}
}

func TestGetChangesReport_StateChanges(t *testing.T) {
	dbPath := "/tmp/test_reports_states.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create host
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'test-host', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	// Insert container in running state (4 days ago)
	fourDaysAgo := time.Now().Add(-4 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "app123", "my-app", "node:18", "sha256:xyz", "running", "Up", fourDaysAgo, 1, "test-host", fourDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert running container: %v", err)
	}

	// Insert same container in stopped state (2 days ago)
	twoDaysAgo := time.Now().Add(-2 * 24 * time.Hour)
	_, err = db.conn.Exec(`
		INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "app123", "my-app", "node:18", "sha256:xyz", "exited", "Exited (0)", twoDaysAgo, 1, "test-host", twoDaysAgo)
	if err != nil {
		t.Fatalf("Failed to insert stopped container: %v", err)
	}

	// Test: Should detect state change
	start := time.Now().Add(-7 * 24 * time.Hour)
	end := time.Now()
	report, err := db.GetChangesReport(start, end, 0)
	if err != nil {
		t.Fatalf("GetChangesReport failed: %v", err)
	}

	if len(report.StateChanges) != 1 {
		t.Errorf("Expected 1 state change, got %d", len(report.StateChanges))
	}

	if len(report.StateChanges) > 0 {
		s := report.StateChanges[0]
		if s.ContainerName != "my-app" {
			t.Errorf("Expected container name 'my-app', got '%s'", s.ContainerName)
		}
		if s.OldState != "running" {
			t.Errorf("Expected old state 'running', got '%s'", s.OldState)
		}
		if s.NewState != "exited" {
			t.Errorf("Expected new state 'exited', got '%s'", s.NewState)
		}
	}
}

func TestGetChangesReport_SummaryAccuracy(t *testing.T) {
	dbPath := "/tmp/test_reports_summary.db"
	defer os.Remove(dbPath)

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create 2 hosts
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'host1', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host1: %v", err)
	}
	_, err = db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (2, 'host2', 'tcp://host2:2376', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert host2: %v", err)
	}

	// Add containers across both hosts
	now := time.Now()
	containers := []struct {
		id       string
		name     string
		hostID   int64
		hostName string
		scanTime time.Time
	}{
		{"c1", "container1", 1, "host1", now.Add(-5 * 24 * time.Hour)},
		{"c2", "container2", 1, "host1", now.Add(-3 * 24 * time.Hour)},
		{"c3", "container3", 2, "host2", now.Add(-4 * 24 * time.Hour)},
	}

	for _, c := range containers {
		_, err = db.conn.Exec(`
			INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, c.id, c.name, "test:latest", "sha256:test", "running", "Up", c.scanTime, c.hostID, c.hostName, c.scanTime)
		if err != nil {
			t.Fatalf("Failed to insert container %s: %v", c.name, err)
		}
	}

	// Test: Check summary counts
	start := time.Now().Add(-7 * 24 * time.Hour)
	end := time.Now()
	report, err := db.GetChangesReport(start, end, 0)
	if err != nil {
		t.Fatalf("GetChangesReport failed: %v", err)
	}

	// Verify summary counts match array lengths
	if report.Summary.NewContainers != len(report.NewContainers) {
		t.Errorf("Summary.NewContainers (%d) != len(NewContainers) (%d)",
			report.Summary.NewContainers, len(report.NewContainers))
	}
	if report.Summary.RemovedContainers != len(report.RemovedContainers) {
		t.Errorf("Summary.RemovedContainers (%d) != len(RemovedContainers) (%d)",
			report.Summary.RemovedContainers, len(report.RemovedContainers))
	}
	if report.Summary.ImageUpdates != len(report.ImageUpdates) {
		t.Errorf("Summary.ImageUpdates (%d) != len(ImageUpdates) (%d)",
			report.Summary.ImageUpdates, len(report.ImageUpdates))
	}
	if report.Summary.StateChanges != len(report.StateChanges) {
		t.Errorf("Summary.StateChanges (%d) != len(StateChanges) (%d)",
			report.Summary.StateChanges, len(report.StateChanges))
	}

	// Verify host count
	if report.Summary.TotalHosts != 2 {
		t.Errorf("Expected 2 total hosts, got %d", report.Summary.TotalHosts)
	}

	// Verify container count
	if report.Summary.TotalContainers != 3 {
		t.Errorf("Expected 3 total containers, got %d", report.Summary.TotalContainers)
	}
}

// Helper function to setup test data
func setupReportTestData(t *testing.T, db *DB) {
	// Create test hosts
	_, err := db.conn.Exec(`INSERT INTO hosts (id, name, address, enabled) VALUES (1, 'test-host-1', 'unix:///var/run/docker.sock', 1)`)
	if err != nil {
		t.Fatalf("Failed to insert test host: %v", err)
	}

	// Insert some test containers at different times
	times := []time.Time{
		time.Now().Add(-10 * 24 * time.Hour),
		time.Now().Add(-5 * 24 * time.Hour),
		time.Now().Add(-2 * 24 * time.Hour),
	}

	for i, ts := range times {
		_, err = db.conn.Exec(`
			INSERT INTO containers (id, name, image, image_id, state, status, created, host_id, host_name, scanned_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			"container"+string(rune(i)),
			"test-container-"+string(rune(i)),
			"nginx:latest",
			"sha256:test"+string(rune(i)),
			"running",
			"Up",
			ts,
			1,
			"test-host-1",
			ts,
		)
		if err != nil {
			t.Fatalf("Failed to insert test container: %v", err)
		}
	}
}
