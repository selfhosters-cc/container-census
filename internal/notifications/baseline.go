package notifications

import (
	"context"
	"log"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

// BaselineCollector manages baseline statistics for anomaly detection
type BaselineCollector struct {
	db *storage.DB
}

// NewBaselineCollector creates a new baseline collector
func NewBaselineCollector(db *storage.DB) *BaselineCollector {
	return &BaselineCollector{
		db: db,
	}
}

// UpdateBaselines updates baseline stats for all containers
// This should be called hourly to maintain 48-hour rolling baselines
func (bc *BaselineCollector) UpdateBaselines(ctx context.Context) error {
	log.Printf("Starting baseline stats update...")

	// Get all hosts
	hosts, err := bc.db.GetHosts()
	if err != nil {
		return err
	}

	var updated, failed int

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}

		// Update baselines for this host
		count, err := bc.updateHostBaselines(ctx, host.ID)
		if err != nil {
			log.Printf("Failed to update baselines for host %s: %v", host.Name, err)
			failed++
			continue
		}
		updated += count
	}

	log.Printf("Baseline update complete: %d updated, %d failed", updated, failed)
	return nil
}

// updateHostBaselines updates baselines for all containers on a specific host
func (bc *BaselineCollector) updateHostBaselines(ctx context.Context, hostID int64) (int, error) {
	// Get current containers for this host
	containers, err := bc.db.GetContainersByHost(hostID)
	if err != nil {
		return 0, err
	}

	var updated int

	for _, container := range containers {
		// Calculate baseline for last 48 hours
		baseline, err := bc.calculateBaseline(container.ID, hostID, container.Name, container.ImageID)
		if err != nil {
			log.Printf("Failed to calculate baseline for %s: %v", container.Name, err)
			continue
		}

		if baseline == nil {
			// Not enough data
			continue
		}

		// Save baseline
		if err := bc.db.SaveContainerBaseline(baseline); err != nil {
			log.Printf("Failed to save baseline for %s: %v", container.Name, err)
			continue
		}

		updated++
	}

	return updated, nil
}

// calculateBaseline calculates baseline stats from the last 48 hours
func (bc *BaselineCollector) calculateBaseline(containerID string, hostID int64, containerName, imageID string) (*models.ContainerBaselineStats, error) {
	// Get stats from last 48 hours
	windowStart := time.Now().Add(-48 * time.Hour)
	windowEnd := time.Now()

	stats, err := bc.db.GetContainerStats(containerID, hostID, 48)
	if err != nil {
		return nil, err
	}

	// Need at least 10 samples to create a meaningful baseline
	if len(stats) < 10 {
		return nil, nil
	}

	// Calculate averages
	var totalCPU, totalMemoryPercent float64
	var totalMemoryUsage int64
	var sampleCount int

	for _, point := range stats {
		// Only include non-zero values
		if point.CPUPercent > 0 || point.MemoryUsage > 0 {
			totalCPU += point.CPUPercent
			totalMemoryPercent += point.MemoryPercent
			totalMemoryUsage += point.MemoryUsage
			sampleCount++
		}
	}

	if sampleCount == 0 {
		return nil, nil
	}

	avgCPU := totalCPU / float64(sampleCount)
	avgMemoryPercent := totalMemoryPercent / float64(sampleCount)
	avgMemoryUsage := totalMemoryUsage / int64(sampleCount)

	baseline := &models.ContainerBaselineStats{
		ContainerID:      containerID,
		ContainerName:    containerName,
		HostID:           hostID,
		ImageID:          imageID,
		AvgCPUPercent:    avgCPU,
		AvgMemoryPercent: avgMemoryPercent,
		AvgMemoryUsage:   avgMemoryUsage,
		SampleCount:      sampleCount,
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
	}

	return baseline, nil
}

// UpdateBaselineOnImageChange updates baseline immediately when an image is updated
// This captures the "before" state for anomaly detection
func (bc *BaselineCollector) UpdateBaselineOnImageChange(containerID string, hostID int64, containerName, oldImageID string) error {
	log.Printf("Capturing baseline for %s before image update", containerName)

	baseline, err := bc.calculateBaseline(containerID, hostID, containerName, oldImageID)
	if err != nil {
		return err
	}

	if baseline == nil {
		// Not enough data
		log.Printf("Not enough data to create baseline for %s", containerName)
		return nil
	}

	return bc.db.SaveContainerBaseline(baseline)
}

// StartPeriodicUpdates starts a background goroutine that updates baselines hourly
func (bc *BaselineCollector) StartPeriodicUpdates(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run once immediately
	if err := bc.UpdateBaselines(ctx); err != nil {
		log.Printf("Initial baseline update failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := bc.UpdateBaselines(ctx); err != nil {
				log.Printf("Periodic baseline update failed: %v", err)
			}
		case <-ctx.Done():
			log.Printf("Baseline collector stopped")
			return
		}
	}
}
