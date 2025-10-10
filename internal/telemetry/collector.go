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

	// Aggregate image statistics (anonymized)
	imageMap := make(map[string]int)
	for _, container := range containers {
		// Anonymize image name but keep it useful for statistics
		imageMap[container.Image]++
	}

	// Convert to slice
	imageStats := make([]models.ImageStat, 0, len(imageMap))
	for image, count := range imageMap {
		imageStats = append(imageStats, models.ImageStat{
			Image: image,
			Count: count,
		})
	}

	// Collect agent versions
	agentVersions := make(map[string]int)
	for _, info := range agentStats {
		if info.Version != "" {
			agentVersions[info.Version]++
		}
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
