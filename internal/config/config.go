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
