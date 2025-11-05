package migration

import (
	"fmt"
	"log"
	"os"

	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
)

// ImportYAMLConfig imports settings from config.yaml to database (one-time migration)
func ImportYAMLConfig(yamlPath string, db *storage.DB) error {
	// 1. Check if already migrated
	if !db.IsFirstRun() {
		return fmt.Errorf("settings already exist in database, migration not needed")
	}

	log.Printf("Starting configuration migration from %s", yamlPath)

	// 2. Try to read YAML file
	cfg, err := config.Load(yamlPath)
	if err != nil {
		// If file doesn't exist or can't be read, use defaults
		if os.IsNotExist(err) {
			log.Printf("No config.yaml found at %s, using default settings", yamlPath)
		} else {
			log.Printf("Failed to read config.yaml: %v, using default settings", err)
		}

		// Save default settings
		defaults := storage.GetDefaultSettings()
		if err := db.SaveSystemSettings(defaults); err != nil {
			return fmt.Errorf("failed to save default settings: %w", err)
		}

		// Auto-create local Unix socket host on fresh install
		if err := createDefaultLocalHost(db); err != nil {
			log.Printf("Warning: failed to create default local host: %v", err)
		}

		log.Println("✅ Default configuration initialized in database")
		return nil
	}

	// 3. Convert YAML → SystemSettings
	settings := convertConfigToSettings(cfg)

	// 4. Save to database
	if err := db.SaveSystemSettings(settings); err != nil {
		return fmt.Errorf("failed to save settings to database: %w", err)
	}

	// 5. Import hosts (if any)
	if err := importHosts(cfg.Hosts, db); err != nil {
		log.Printf("Warning: failed to import some hosts: %v", err)
	}

	// 6. Import telemetry endpoints
	if err := importTelemetryEndpoints(cfg.Telemetry.Endpoints, db); err != nil {
		log.Printf("Warning: failed to import telemetry endpoints: %v", err)
	}

	// 7. Set migration flag for UI notification
	if err := db.SetPreference("config_migrated", "true"); err != nil {
		log.Printf("Warning: failed to set migration flag: %v", err)
	}

	log.Println("✅ Configuration migrated from config.yaml to database successfully")
	log.Println("   config.yaml is no longer used. Settings are now stored in the database.")

	return nil
}

// convertConfigToSettings converts Config struct to SystemSettings
func convertConfigToSettings(cfg *models.Config) *models.SystemSettings {
	return &models.SystemSettings{
		Scanner: models.ScannerSettings{
			IntervalSeconds: cfg.Scanner.IntervalSeconds,
			TimeoutSeconds:  cfg.Scanner.TimeoutSeconds,
		},
		Telemetry: models.TelemetrySettings{
			IntervalHours: cfg.Telemetry.IntervalHours,
		},
		Notification: models.NotificationSettings{
			RateLimitMax:           100,  // Default, not in YAML
			RateLimitBatchInterval: 600,  // Default, not in YAML
			ThresholdDuration:      120,  // Default, not in YAML
			CooldownPeriod:         300,  // Default, not in YAML
		},
	}
}

// importHosts imports hosts from YAML to database
func importHosts(yamlHosts []models.HostConfig, db *storage.DB) error {
	if len(yamlHosts) == 0 {
		log.Println("No hosts to import from config.yaml")
		return nil
	}

	// Check if hosts already exist in database
	existingHosts, err := db.GetHosts()
	if err != nil {
		return fmt.Errorf("failed to check existing hosts: %w", err)
	}

	// If hosts already exist, don't overwrite them
	if len(existingHosts) > 0 {
		log.Printf("Hosts already exist in database (%d hosts), skipping host import", len(existingHosts))
		return nil
	}

	// Import each host
	imported := 0
	for _, hostCfg := range yamlHosts {
		host := &models.Host{
			Name:         hostCfg.Name,
			Address:      hostCfg.Address,
			Description:  hostCfg.Description,
			Enabled:      true,
			CollectStats: true,
		}

		if _, err := db.AddHost(*host); err != nil {
			log.Printf("Warning: failed to import host %s: %v", hostCfg.Name, err)
			continue
		}

		imported++
	}

	log.Printf("Imported %d host(s) from config.yaml", imported)
	return nil
}

// importTelemetryEndpoints imports telemetry endpoints from YAML to database
func importTelemetryEndpoints(yamlEndpoints []models.TelemetryEndpoint, db *storage.DB) error {
	if len(yamlEndpoints) == 0 {
		log.Println("No telemetry endpoints to import from config.yaml")
		return nil
	}

	// Check if endpoints already exist
	existingEndpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		return fmt.Errorf("failed to check existing endpoints: %w", err)
	}

	// If endpoints already exist, don't overwrite
	if len(existingEndpoints) > 0 {
		log.Printf("Telemetry endpoints already exist in database (%d endpoints), skipping import", len(existingEndpoints))
		return nil
	}

	// Import each endpoint
	imported := 0
	for _, ep := range yamlEndpoints {
		endpoint := &models.TelemetryEndpoint{
			Name:    ep.Name,
			URL:     ep.URL,
			Enabled: ep.Enabled,
			APIKey:  ep.APIKey,
		}

		if err := db.SaveTelemetryEndpoint(endpoint); err != nil {
			log.Printf("Warning: failed to import telemetry endpoint %s: %v", ep.Name, err)
			continue
		}

		imported++
	}

	log.Printf("Imported %d telemetry endpoint(s) from config.yaml", imported)
	return nil
}

// createDefaultLocalHost creates a default local Unix socket host on fresh install
func createDefaultLocalHost(db *storage.DB) error {
	// Check if any hosts already exist
	existingHosts, err := db.GetHosts()
	if err != nil {
		return fmt.Errorf("failed to check existing hosts: %w", err)
	}

	if len(existingHosts) > 0 {
		log.Printf("Hosts already exist in database, skipping default local host creation")
		return nil
	}

	// Create default local host
	localHost := &models.Host{
		Name:         "local",
		Address:      "unix:///var/run/docker.sock",
		Description:  "Local Docker daemon",
		Enabled:      true,
		CollectStats: true,
	}

	if _, err := db.AddHost(*localHost); err != nil {
		return fmt.Errorf("failed to create local host: %w", err)
	}

	log.Println("✅ Created default local Docker host (unix:///var/run/docker.sock)")
	return nil
}
