package config

import (
	"fmt"
	"os"

	"github.com/container-census/container-census/internal/models"
	"gopkg.in/yaml.v3"
)

// Load reads and parses the configuration file
func Load(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg models.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Apply defaults
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/census.db"
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	if cfg.Scanner.IntervalSeconds == 0 {
		cfg.Scanner.IntervalSeconds = 300 // 5 minutes
	}

	if cfg.Scanner.TimeoutSeconds == 0 {
		cfg.Scanner.TimeoutSeconds = 30
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *models.Config) {
	// Database path override
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		cfg.Database.Path = dbPath
	}

	// Server host override
	if host := os.Getenv("SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}

	// Server port override
	if portStr := os.Getenv("SERVER_PORT"); portStr != "" {
		var port int
		if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil && port > 0 {
			cfg.Server.Port = port
		}
	}

	// Scanner interval override
	if intervalStr := os.Getenv("SCANNER_INTERVAL_SECONDS"); intervalStr != "" {
		var interval int
		if _, err := fmt.Sscanf(intervalStr, "%d", &interval); err == nil && interval > 0 {
			cfg.Scanner.IntervalSeconds = interval
		}
	}

	// Telemetry enabled override
	if telemetryStr := os.Getenv("TELEMETRY_ENABLED"); telemetryStr != "" {
		cfg.Telemetry.Enabled = telemetryStr == "true" || telemetryStr == "1" || telemetryStr == "yes"
	}

	// Telemetry interval override
	if intervalStr := os.Getenv("TELEMETRY_INTERVAL_HOURS"); intervalStr != "" {
		var interval int
		if _, err := fmt.Sscanf(intervalStr, "%d", &interval); err == nil && interval > 0 {
			cfg.Telemetry.IntervalHours = interval
		}
	}
}

// LoadOrDefault loads config from file or returns default config
func LoadOrDefault(path string) *models.Config {
	cfg, err := Load(path)
	if err != nil {
		// Return default config
		return &models.Config{
			Database: models.DatabaseConfig{
				Path: "./data/census.db",
			},
			Server: models.ServerConfig{
				Host: "0.0.0.0",
				Port: 8080,
			},
			Scanner: models.ScannerConfig{
				IntervalSeconds: 300,
				TimeoutSeconds:  30,
			},
			Hosts: []models.HostConfig{
				{
					Name:        "local",
					Address:     "unix:///var/run/docker.sock",
					Description: "Local Docker daemon",
				},
			},
		}
	}
	return cfg
}
