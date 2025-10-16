package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/version"
	"github.com/google/uuid"
)

const (
	installationIDFile = "./data/.installation_id"
)

// Collector gathers anonymous telemetry data
type Collector struct {
	db               *storage.DB
	installationID   string
	scanInterval     int
}

// NewCollector creates a new telemetry collector
func NewCollector(db *storage.DB, scanInterval int) (*Collector, error) {
	installID, err := getOrCreateInstallationID()
	if err != nil {
		return nil, fmt.Errorf("failed to get installation ID: %w", err)
	}

	return &Collector{
		db:             db,
		installationID: installID,
		scanInterval:   scanInterval,
	}, nil
}

// CollectReport generates a telemetry report from current data
func (c *Collector) CollectReport(ctx context.Context, agentStats map[string]*models.AgentInfo) (*models.TelemetryReport, error) {
	// Get all hosts
	hosts, err := c.db.GetHosts()
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts: %w", err)
	}

	// Count enabled hosts and agents
	enabledHosts := 0
	agentCount := 0
	for _, host := range hosts {
		if host.Enabled {
			enabledHosts++
			if host.HostType == "agent" {
				agentCount++
			}
		}
	}

	// Get latest containers
	containers, err := c.db.GetLatestContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	// Aggregate image statistics and collect sizes
	imageMap := make(map[string]*models.ImageStat)
	for _, container := range containers {
		if stat, exists := imageMap[container.Image]; exists {
			stat.Count++
			// Add size if not already counted for this image
			if container.ImageSize > 0 && stat.SizeBytes == 0 {
				stat.SizeBytes = container.ImageSize
			}
		} else {
			imageMap[container.Image] = &models.ImageStat{
				Image:     container.Image,
				Count:     1,
				SizeBytes: container.ImageSize,
			}
		}
	}

	// Convert to slice
	imageStats := make([]models.ImageStat, 0, len(imageMap))
	totalImageSize := int64(0)
	for _, stat := range imageMap {
		imageStats = append(imageStats, *stat)
		totalImageSize += stat.SizeBytes
	}

	// Collect agent versions
	agentVersions := make(map[string]int)
	for _, info := range agentStats {
		if info.Version != "" {
			agentVersions[info.Version]++
		}
	}

	// Aggregate container states
	containersRunning := 0
	containersStopped := 0
	containersPaused := 0
	containersOther := 0

	// Aggregate resource usage (for running containers)
	var totalCPU float64
	var totalMemory int64
	var totalMemoryLimit int64
	runningContainersCount := 0

	// Aggregate restart statistics
	var totalRestarts int
	highRestartContainers := 0

	for _, container := range containers {
		// Count states
		switch container.State {
		case "running":
			containersRunning++
			// Collect resource stats for running containers
			if container.CPUPercent > 0 || container.MemoryUsage > 0 {
				totalCPU += container.CPUPercent
				totalMemory += container.MemoryUsage
				totalMemoryLimit += container.MemoryLimit
				runningContainersCount++
			}
		case "exited":
			containersStopped++
		case "paused":
			containersPaused++
		default:
			containersOther++
		}

		// Collect restart stats
		totalRestarts += container.RestartCount
		if container.RestartCount > 10 {
			highRestartContainers++
		}
	}

	// Calculate averages
	avgCPU := 0.0
	avgMemory := int64(0)
	if runningContainersCount > 0 {
		avgCPU = totalCPU / float64(runningContainersCount)
		avgMemory = totalMemory / int64(runningContainersCount)
	}

	avgRestarts := 0.0
	if len(containers) > 0 {
		avgRestarts = float64(totalRestarts) / float64(len(containers))
	}

	// Get system timezone
	timezone := "UTC"
	if tz := os.Getenv("TZ"); tz != "" {
		timezone = tz
	}

	report := &models.TelemetryReport{
		InstallationID:  c.installationID,
		Version:         version.Get(),
		Timestamp:       time.Now().UTC(),
		HostCount:       enabledHosts,
		AgentCount:      agentCount,
		TotalContainers: len(containers),
		ScanInterval:    c.scanInterval,
		ImageStats:      imageStats,
		AgentVersions:   agentVersions,
		// New fields
		ContainersRunning:     containersRunning,
		ContainersStopped:     containersStopped,
		ContainersPaused:      containersPaused,
		ContainersOther:       containersOther,
		AvgCPUPercent:         avgCPU,
		AvgMemoryBytes:        avgMemory,
		TotalMemoryLimit:      totalMemoryLimit,
		AvgRestarts:           avgRestarts,
		HighRestartContainers: highRestartContainers,
		TotalImageSize:        totalImageSize,
		UniqueImages:          len(imageMap),
		Timezone:              timezone,
	}

	return report, nil
}

// getOrCreateInstallationID gets or creates a unique installation ID
func getOrCreateInstallationID() (string, error) {
	// Try to read existing ID
	data, err := os.ReadFile(installationIDFile)
	if err == nil {
		id := string(data)
		if id != "" {
			return id, nil
		}
	}

	// Generate new UUID
	newID := uuid.New().String()

	// Try to save it
	if err := os.WriteFile(installationIDFile, []byte(newID), 0644); err != nil {
		log.Printf("Warning: failed to save installation ID: %v", err)
		// Continue with in-memory ID
	}

	return newID, nil
}

// hashString creates a SHA256 hash of a string (for anonymization if needed)
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
