package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/container-census/container-census/internal/api"
	"github.com/container-census/container-census/internal/auth"
	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/telemetry"
	"github.com/container-census/container-census/internal/version"
)

func main() {
	log.Printf("Starting Container Census v%s...", version.Get())

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config/config.yaml"
	}

	cfg, fromFile := config.LoadOrDefault(configPath)
	if fromFile {
		log.Printf("Configuration loaded from file: %s", configPath)
	} else {
		log.Printf("Using default configuration (config file not found, using environment variables)")
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Initialize database
	db, err := storage.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized: %s", cfg.Database.Path)

	// Initialize hosts from config if database is empty
	if err := initializeHosts(db, cfg.Hosts); err != nil {
		log.Printf("Warning: Failed to initialize hosts: %v", err)
	}

	// Initialize scanner
	scan := scanner.New(cfg.Scanner.TimeoutSeconds)
	log.Println("Scanner initialized")

	// Initialize API server with authentication config
	authConfig := convertAuthConfig(cfg.Server.Auth)
	if authConfig.Enabled {
		log.Printf("Authentication enabled for user: %s", authConfig.Username)
	} else {
		log.Println("Authentication disabled - UI and API are publicly accessible")
	}

	apiServer := api.New(db, scan, configPath, cfg.Scanner.IntervalSeconds, authConfig)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      apiServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start background scanner
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runPeriodicScans(ctx, db, scan, cfg.Scanner.IntervalSeconds)

	// Start telemetry scheduler if any endpoint is enabled
	enabledCount := 0
	for _, ep := range cfg.Telemetry.Endpoints {
		if ep.Enabled {
			enabledCount++
		}
	}
	if enabledCount > 0 {
		telemetryScheduler, err := telemetry.NewScheduler(db, scan, cfg.Telemetry, cfg.Scanner.IntervalSeconds)
		if err != nil {
			log.Printf("Warning: Failed to initialize telemetry: %v", err)
		} else {
			telemetryCtx, telemetryCancel := context.WithCancel(ctx)
			apiServer.SetTelemetryScheduler(telemetryScheduler, telemetryCtx, telemetryCancel)
			go telemetryScheduler.Start(telemetryCtx)
		}
	}

	// Start HTTP server
	go func() {
		log.Printf("Server listening on http://%s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	cancel() // Stop background scanner

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// convertAuthConfig converts models.AuthConfig to auth.Config
func convertAuthConfig(cfg models.AuthConfig) auth.Config {
	return auth.Config{
		Enabled:  cfg.Enabled,
		Username: cfg.Username,
		Password: cfg.Password,
	}
}

// initializeHosts adds hosts from config to database if they don't exist
func initializeHosts(db *storage.DB, hostsConfig []models.HostConfig) error {
	existingHosts, err := db.GetHosts()
	if err != nil {
		return err
	}

	// If hosts already exist in database, don't add from config
	if len(existingHosts) > 0 {
		return nil
	}

	// Add hosts from config
	for _, hc := range hostsConfig {
		host := models.Host{
			Name:        hc.Name,
			Address:     hc.Address,
			Description: hc.Description,
			HostType:    detectHostType(hc.Address),
			Enabled:     true,
		}

		id, err := db.AddHost(host)
		if err != nil {
			log.Printf("Failed to add host %s: %v", hc.Name, err)
			continue
		}
		log.Printf("Added host: %s (ID: %d, Type: %s)", hc.Name, id, host.HostType)
	}

	return nil
}

// detectHostType determines the host type from its address
func detectHostType(address string) string {
	switch {
	case address == "" || address == "local":
		return "unix"
	case len(address) >= 7 && address[:7] == "agent://":
		return "agent"
	case len(address) >= 7 && address[:7] == "http://":
		return "agent"
	case len(address) >= 8 && address[:8] == "https://":
		return "agent"
	case len(address) >= 7 && address[:7] == "unix://":
		return "unix"
	case len(address) >= 6 && address[:6] == "tcp://":
		return "tcp"
	case len(address) >= 6 && address[:6] == "ssh://":
		return "ssh"
	default:
		return "unknown"
	}
}

// runPeriodicScans runs scans at regular intervals
func runPeriodicScans(ctx context.Context, db *storage.DB, scan *scanner.Scanner, intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	// Run initial scan
	log.Println("Running initial scan...")
	performScan(ctx, db, scan)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping periodic scans")
			return
		case <-ticker.C:
			log.Println("Running periodic scan...")
			performScan(ctx, db, scan)
		}
	}
}

// performScan executes a scan of all enabled hosts
func performScan(ctx context.Context, db *storage.DB, scan *scanner.Scanner) {
	hosts, err := db.GetHosts()
	if err != nil {
		log.Printf("Failed to get hosts: %v", err)
		return
	}

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}

		result := models.ScanResult{
			HostID:    host.ID,
			HostName:  host.Name,
			StartedAt: time.Now(),
		}

		containers, err := scan.ScanHost(ctx, host)
		result.CompletedAt = time.Now()

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			log.Printf("Scan failed for host %s: %v", host.Name, err)
		} else {
			result.Success = true
			result.ContainersFound = len(containers)
			log.Printf("Scan completed for host %s: found %d containers", host.Name, len(containers))

			// Save containers
			if err := db.SaveContainers(containers); err != nil {
				log.Printf("Failed to save containers for host %s: %v", host.Name, err)
			}
		}

		// Save scan result
		if _, err := db.SaveScanResult(result); err != nil {
			log.Printf("Failed to save scan result for host %s: %v", host.Name, err)
		}
	}
}
