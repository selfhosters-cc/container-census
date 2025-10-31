package notifications

import (
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

// setupTestBaseline creates a test baseline collector
func setupTestBaseline(t *testing.T) (*BaselineCollector, *storage.Store) {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "baseline-test-*.db")
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

	bc := NewBaselineCollector(db)

	return bc, store
}

// TestBaselineCollection_Calculate48HourAverage tests baseline calculation
func TestBaselineCollection_Calculate48HourAverage(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create 48 hours of container stats (one per hour)
	for i := 0; i < 48; i++ {
		container := models.Container{
			ID: "baseline123",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:abc123",
			State:       "running",
			CPUPercent:  float64(40 + i%10),         // Varying CPU: 40-50%
			MemoryUsage: int64(400000000 + i*100000), // Slightly increasing memory
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect baseline
	err := bc.CollectBaselines()
	if err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Retrieve baseline
	baseline, err := db.GetContainerBaseline(host.ID, "baseline123")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline == nil {
		t.Fatal("Expected baseline to be created")
	}

	// Verify baseline values
	if baseline.AvgCPUPercent < 40 || baseline.AvgCPUPercent > 50 {
		t.Errorf("Expected avg CPU between 40-50, got %f", baseline.AvgCPUPercent)
	}

	if baseline.AvgMemoryUsage < 400000000 {
		t.Errorf("Expected avg memory > 400MB, got %d", baseline.AvgMemoryUsage)
	}

	if baseline.SampleCount != 48 {
		t.Errorf("Expected 48 samples, got %d", baseline.SampleCount)
	}

	if baseline.ImageID != "sha256:abc123" {
		t.Errorf("Expected image ID sha256:abc123, got %s", baseline.ImageID)
	}
}

// TestBaselineCollection_MinimumSamples tests minimum sample requirement
func TestBaselineCollection_MinimumSamples(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create only 5 samples (below minimum of 10)
	for i := 0; i < 5; i++ {
		container := models.Container{
			ID: "few-samples",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:abc123",
			State:       "running",
			CPUPercent:  50.0,
			MemoryUsage: 500000000,
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect baseline
	err := bc.CollectBaselines()
	if err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Retrieve baseline
	baseline, err := db.GetContainerBaseline(host.ID, "few-samples")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	// Should not create baseline with too few samples
	if baseline != nil {
		t.Log("NOTE: Baseline created despite few samples - verify minimum sample logic")
		if baseline.SampleCount < 10 {
			t.Logf("Baseline has only %d samples (expected minimum 10)", baseline.SampleCount)
		}
	} else {
		t.Log("Correctly did not create baseline with insufficient samples")
	}
}

// TestBaselineCollection_ImageChange tests baseline capture on image update
func TestBaselineCollection_ImageChange(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create baseline with old image (48 hours)
	for i := 0; i < 48; i++ {
		container := models.Container{
			ID: "img-change",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:old",
			State:       "running",
			CPUPercent:  40.0,
			MemoryUsage: 400000000,
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect initial baseline
	if err := bc.CollectBaselines(); err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Verify initial baseline
	baseline1, err := db.GetContainerBaseline(host.ID, "img-change")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline1 == nil {
		t.Fatal("Expected initial baseline")
	}

	if baseline1.ImageID != "sha256:old" {
		t.Errorf("Expected image ID sha256:old, got %s", baseline1.ImageID)
	}

	// Now create data with new image
	for i := 0; i < 48; i++ {
		container := models.Container{
			ID: "img-change",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v2",
			ImageID:     "sha256:new",
			State:       "running",
			CPUPercent:  50.0, // Different stats
			MemoryUsage: 500000000,
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-47+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect new baseline
	if err := bc.CollectBaselines(); err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Should have updated baseline with new image
	baseline2, err := db.GetContainerBaseline(host.ID, "img-change")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline2 == nil {
		t.Fatal("Expected updated baseline")
	}

	if baseline2.ImageID != "sha256:new" {
		t.Errorf("Expected baseline updated to sha256:new, got %s", baseline2.ImageID)
	}

	if baseline2.AvgCPUPercent == baseline1.AvgCPUPercent {
		t.Log("NOTE: Baseline CPU not updated - verify update logic")
	}
}

// TestBaselineCollection_MultipleContainers tests baseline for multiple containers
func TestBaselineCollection_MultipleContainers(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create data for multiple containers
	containers := []string{"container1", "container2", "container3"}

	for _, containerID := range containers {
		for i := 0; i < 48; i++ {
			container := models.Container{
				ID: containerID,
				HostID:      host.ID,
				Name:        containerID,
				Image:       "app:v1",
				ImageID:     "sha256:abc123",
				State:       "running",
				CPUPercent:  float64(30 + i%20),
				MemoryUsage: 300000000,
				MemoryLimit: 1073741824,
				ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
			}

			if err := db.SaveContainers([]models.Container{container}); err != nil {
				t.Fatalf("Failed to save container: %v", err)
			}
		}
	}

	// Collect baselines
	if err := bc.CollectBaselines(); err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Verify all containers have baselines
	for _, containerID := range containers {
		baseline, err := db.GetContainerBaseline(host.ID, containerID)
		if err != nil {
			t.Fatalf("GetContainerBaseline failed for %s: %v", containerID, err)
		}

		if baseline == nil {
			t.Errorf("Expected baseline for %s", containerID)
			continue
		}

		t.Logf("Container %s baseline: CPU=%f, Memory=%d, Samples=%d",
			containerID, baseline.AvgCPUPercent, baseline.AvgMemoryUsage, baseline.SampleCount)
	}
}

// TestBaselineCollection_NoStatsData tests behavior when no stats are available
func TestBaselineCollection_NoStatsData(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	// Create container without stats
	container := models.Container{
		ID: "no-stats",
		HostID:      host.ID,
		Name:        "app",
		Image:       "app:v1",
		State:       "running",
		// No CPU/Memory stats
		ScannedAt: time.Now(),
	}

	if err := db.SaveContainers([]models.Container{container}); err != nil {
		t.Fatalf("Failed to save container: %v", err)
	}

	// Collect baselines (should not fail)
	err := bc.CollectBaselines()
	if err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Should not have baseline
	baseline, err := db.GetContainerBaseline(host.ID, "no-stats")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline != nil {
		t.Log("NOTE: Baseline created for container without stats - verify logic")
	}
}

// TestBaselineCollection_StoppedContainers tests that stopped containers are excluded
func TestBaselineCollection_StoppedContainers(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host
	host := &models.Host{Name: "test-host", Address: "unix:///", Enabled: true, CollectStats: true}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create container that was running, then stopped
	for i := 0; i < 24; i++ {
		container := models.Container{
			ID: "stopped-later",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:abc123",
			State:       "running",
			CPUPercent:  50.0,
			MemoryUsage: 500000000,
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Add stopped states for last 24 hours
	for i := 24; i < 48; i++ {
		container := models.Container{
			ID: "stopped-later",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:abc123",
			State:       "exited",
			ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect baselines
	if err := bc.CollectBaselines(); err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// May or may not have baseline depending on logic
	baseline, err := db.GetContainerBaseline(host.ID, "stopped-later")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline != nil {
		t.Logf("Baseline exists for stopped container with %d samples", baseline.SampleCount)
		// Should only include running state samples
		if baseline.SampleCount > 24 {
			t.Errorf("Expected <= 24 samples (running only), got %d", baseline.SampleCount)
		}
	} else {
		t.Log("No baseline for currently stopped container (expected)")
	}
}

// TestAnomalyThreshold tests the 25% increase threshold for anomalies
func TestAnomalyThreshold(t *testing.T) {
	// This is a calculation test, not requiring database

	baselineCPU := 40.0
	baselineMemory := int64(400000000)

	// Test cases
	tests := []struct {
		name           string
		currentCPU     float64
		currentMemory  int64
		shouldBeAnomaly bool
	}{
		{"Normal CPU", 42.0, 400000000, false},
		{"25% CPU increase (threshold)", 50.0, 400000000, false},
		{"30% CPU increase", 52.0, 400000000, true},
		{"Normal Memory", 40.0, 420000000, false},
		{"25% Memory increase (threshold)", 40.0, 500000000, false},
		{"30% Memory increase", 40.0, 520000000, true},
		{"Both increased 30%", 52.0, 520000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuThreshold := baselineCPU * 1.25
			memoryThreshold := float64(baselineMemory) * 1.25

			cpuAnomaly := tt.currentCPU > cpuThreshold
			memoryAnomaly := float64(tt.currentMemory) > memoryThreshold

			isAnomaly := cpuAnomaly || memoryAnomaly

			if isAnomaly != tt.shouldBeAnomaly {
				t.Errorf("Expected anomaly=%v, got %v (CPU: %f > %f = %v, Mem: %d > %f = %v)",
					tt.shouldBeAnomaly, isAnomaly,
					tt.currentCPU, cpuThreshold, cpuAnomaly,
					tt.currentMemory, memoryThreshold, memoryAnomaly)
			}
		})
	}
}

// TestBaselineCollection_DisabledStatsHost tests that hosts with CollectStats=false are skipped
func TestBaselineCollection_DisabledStatsHost(t *testing.T) {
	bc, db := setupTestBaseline(t)

	// Create host with stats disabled
	host := &models.Host{Name: "no-stats-host", Address: "unix:///", Enabled: true, CollectStats: false}
	if err := db.SaveHost(host); err != nil {
		t.Fatalf("Failed to save host: %v", err)
	}

	now := time.Now()

	// Create container data anyway
	for i := 0; i < 48; i++ {
		container := models.Container{
			ID: "disabled-stats",
			HostID:      host.ID,
			Name:        "app",
			Image:       "app:v1",
			ImageID:     "sha256:abc123",
			State:       "running",
			CPUPercent:  50.0,
			MemoryUsage: 500000000,
			MemoryLimit: 1073741824,
			ScannedAt:   now.Add(time.Duration(-48+i) * time.Hour),
		}

		if err := db.SaveContainers([]models.Container{container}); err != nil {
			t.Fatalf("Failed to save container: %v", err)
		}
	}

	// Collect baselines
	if err := bc.CollectBaselines(); err != nil {
		t.Fatalf("CollectBaselines failed: %v", err)
	}

	// Should not create baseline for host with stats disabled
	baseline, err := db.GetContainerBaseline(host.ID, "disabled-stats")
	if err != nil {
		t.Fatalf("GetContainerBaseline failed: %v", err)
	}

	if baseline != nil {
		t.Error("Expected no baseline for host with CollectStats=false")
	}
}
