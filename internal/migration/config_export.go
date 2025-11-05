package migration

import (
	"fmt"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/vulnerability"
	"gopkg.in/yaml.v3"
)

// ExportToYAML exports database settings to YAML format
func ExportToYAML(db *storage.DB) ([]byte, error) {
	// 1. Load settings from DB
	settings, err := db.LoadSystemSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	// 2. Load hosts
	hosts, err := db.GetHosts()
	if err != nil {
		return nil, fmt.Errorf("failed to load hosts: %w", err)
	}

	// 3. Load telemetry endpoints
	endpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to load telemetry endpoints: %w", err)
	}

	// 4. Load vulnerability settings
	vulnSettings, err := db.LoadVulnerabilitySettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load vulnerability settings: %w", err)
	}

	// 5. Convert to Config struct for YAML export
	cfg := convertSettingsToConfig(settings, hosts, endpoints, vulnSettings)

	// 6. Marshal to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// 7. Add header comment with instructions
	header := `# Container Census Configuration Export
# Generated: ` + time.Now().Format(time.RFC3339) + `
#
# NOTE: Infrastructure settings must be set via environment variables:
#   - DATABASE_PATH: Database file location
#   - SERVER_HOST: Server bind address
#   - SERVER_PORT: Server listen port
#   - AUTH_ENABLED: Enable authentication
#   - AUTH_USERNAME: Username for Basic Auth
#   - AUTH_PASSWORD: Password for Basic Auth
#   - TRIVY_CACHE_DIR: Trivy cache directory
#
# This file can be imported via:
#   - Web UI: Settings → Import Configuration
#   - API: POST /api/settings/import
#   - First run: Place as config/config.yaml before initial startup
#

`

	return []byte(header + string(yamlData)), nil
}

// convertSettingsToConfig converts database settings to Config struct for YAML
func convertSettingsToConfig(settings *models.SystemSettings, hosts []models.Host, endpoints []models.TelemetryEndpoint, vulnSettings *vulnerability.Config) *models.Config {
	// Convert hosts to HostConfig
	var hostConfigs []models.HostConfig
	for _, host := range hosts {
		hostConfigs = append(hostConfigs, models.HostConfig{
			Name:        host.Name,
			Address:     host.Address,
			Description: host.Description,
		})
	}

	// Build config struct
	cfg := &models.Config{
		Database: models.DatabaseConfig{
			Path: "# Set via DATABASE_PATH environment variable",
		},
		Server: models.ServerConfig{
			Host: "# Set via SERVER_HOST environment variable",
			Port: 0, // Set via SERVER_PORT environment variable
			Auth: models.AuthConfig{
				Enabled:  false, // Set via AUTH_ENABLED environment variable
				Username: "# Set via AUTH_USERNAME environment variable",
				Password: "# Set via AUTH_PASSWORD environment variable",
			},
		},
		Scanner: models.ScannerConfig{
			IntervalSeconds: settings.Scanner.IntervalSeconds,
			TimeoutSeconds:  settings.Scanner.TimeoutSeconds,
		},
		Telemetry: models.TelemetryConfig{
			IntervalHours: settings.Telemetry.IntervalHours,
			Endpoints:     endpoints,
		},
		Hosts: hostConfigs,
	}

	// Add vulnerability settings if available
	if vulnSettings != nil {
		cfg.Vulnerability = models.VulnerabilityConfig{
			Enabled:                vulnSettings.GetEnabled(),
			AutoScanNewImages:      vulnSettings.GetAutoScanNewImages(),
			WorkerPoolSize:         vulnSettings.GetWorkerPoolSize(),
			ScanTimeoutMinutes:     int(vulnSettings.GetScanTimeout().Minutes()),
			CacheTTLHours:          int(vulnSettings.GetCacheTTL().Hours()),
			RescanIntervalHours:    int(vulnSettings.GetRescanInterval().Hours()),
			CacheDir:               vulnSettings.GetCacheDir(),
			DBUpdateIntervalHours:  int(vulnSettings.GetDBUpdateInterval().Hours()),
			RetentionDays:          90,  // TODO: Add getter method
			DetailedRetentionDays:  30,  // TODO: Add getter method
			AlertOnCritical:        vulnSettings.GetAlertOnCritical(),
			AlertOnHigh:            vulnSettings.GetAlertOnHigh(),
			MaxQueueSize:           vulnSettings.GetMaxQueueSize(),
		}
	}

	return cfg
}
