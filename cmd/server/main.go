package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/container-census/container-census/internal/api"
	"github.com/container-census/container-census/internal/auth"
	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/notifications"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/telemetry"
	"github.com/container-census/container-census/internal/version"
)

// Global scan interval that can be updated dynamically
var (
	scanIntervalMu     sync.RWMutex
	scanIntervalValue  int
	scanIntervalChange = make(chan int, 1)
)

func getScanInterval() int {
	scanIntervalMu.RLock()
	defer scanIntervalMu.RUnlock()
	return scanIntervalValue
}

func setScanInterval(val int) {
	scanIntervalMu.Lock()
	scanIntervalValue = val
	scanIntervalMu.Unlock()

	// Non-blocking send to notify scanner
	select {
	case scanIntervalChange <- val:
	default:
	}
}

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

	// Initialize default notification channels and rules
	if err := db.InitializeDefaultNotifications(); err != nil {
		log.Printf("Warning: Failed to initialize default notifications: %v", err)
	}

	// Initialize scanner
	scan := scanner.New(cfg.Scanner.TimeoutSeconds)
	log.Println("Scanner initialized")

	// Initialize scan interval
	setScanInterval(cfg.Scanner.IntervalSeconds)
	log.Printf("Scan interval set to %d seconds", cfg.Scanner.IntervalSeconds)

	// Initialize API server with authentication config
	authConfig := convertAuthConfig(cfg.Server.Auth)
	if authConfig.Enabled {
		log.Printf("Authentication enabled for user: %s", authConfig.Username)
	} else {
		log.Println("Authentication disabled - UI and API are publicly accessible")
	}

	apiServer := api.New(db, scan, configPath, cfg.Scanner.IntervalSeconds, authConfig)
	apiServer.SetScanIntervalCallback(setScanInterval) // Allow API to update scan interval dynamically
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

	// Check for updates on startup
	go checkForUpdates()

	// Start daily version check
	go runDailyVersionCheck(ctx)

	// Start daily database cleanup
	go runDailyDatabaseCleanup(ctx, db)

	// Start hourly stats aggregation
	go runHourlyStatsAggregation(ctx, db)

	// Initialize notification system
	maxNotificationsPerHour := getEnvInt("NOTIFICATION_RATE_LIMIT_MAX", 100)
	batchIntervalSeconds := getEnvInt("NOTIFICATION_RATE_LIMIT_BATCH_INTERVAL", 600)
	notificationService := notifications.NewNotificationService(db, maxNotificationsPerHour, time.Duration(batchIntervalSeconds)*time.Second)
	notificationServiceGlobal = notificationService // Set global reference for scanner
	log.Printf("Notification service initialized (rate limit: %d/hour, batch interval: %ds)", maxNotificationsPerHour, batchIntervalSeconds)

	// Pass notification service to API server
	apiServer.SetNotificationService(notificationService)

	// Start baseline stats collector
	baselineCollector := notifications.NewBaselineCollector(db)
	go baselineCollector.StartPeriodicUpdates(ctx)
	log.Println("Baseline stats collector started")

	// Start hourly notification cleanup
	go runHourlyNotificationCleanup(ctx, db)

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

// Global notification service reference
var notificationServiceGlobal *notifications.NotificationService

// runPeriodicScans runs scans at regular intervals
func runPeriodicScans(ctx context.Context, db *storage.DB, scan *scanner.Scanner, intervalSeconds int) {
	currentInterval := getScanInterval()
	ticker := time.NewTicker(time.Duration(currentInterval) * time.Second)
	defer ticker.Stop()

	// Run initial scan
	log.Println("Running initial scan...")
	performScan(ctx, db, scan)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping periodic scans")
			return
		case newInterval := <-scanIntervalChange:
			// Interval changed - recreate ticker
			ticker.Stop()
			ticker = time.NewTicker(time.Duration(newInterval) * time.Second)
			log.Printf("Scan interval changed to %d seconds (will take effect on next scan)", newInterval)
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

			// Update agent status if this is an auth failure
			if host.HostType == "agent" && strings.Contains(err.Error(), "401") {
				host.AgentStatus = "auth_failed"
				if updateErr := db.UpdateHost(host); updateErr != nil {
					log.Printf("Failed to update host status for %s: %v", host.Name, updateErr)
				}
			} else if host.HostType == "agent" {
				// Other failure - mark as offline
				host.AgentStatus = "offline"
				if updateErr := db.UpdateHost(host); updateErr != nil {
					log.Printf("Failed to update host status for %s: %v", host.Name, updateErr)
				}
			}
		} else {
			result.Success = true
			result.ContainersFound = len(containers)
			log.Printf("Scan completed for host %s: found %d containers", host.Name, len(containers))

			// Update agent status to online on successful scan
			if host.HostType == "agent" && host.AgentStatus != "online" {
				host.AgentStatus = "online"
				host.LastSeen = time.Now()
				if updateErr := db.UpdateHost(host); updateErr != nil {
					log.Printf("Failed to update host status for %s: %v", host.Name, updateErr)
				}
			}

			// Save containers
			if err := db.SaveContainers(containers); err != nil {
				log.Printf("Failed to save containers for host %s: %v", host.Name, err)
			}

			// Process notifications for this host
			if notificationServiceGlobal != nil {
				if err := notificationServiceGlobal.ProcessEvents(ctx, host.ID); err != nil {
					log.Printf("Failed to process notifications for host %s: %v", host.Name, err)
				}
			}
		}

		// Save scan result
		if _, err := db.SaveScanResult(result); err != nil {
			log.Printf("Failed to save scan result for host %s: %v", host.Name, err)
		}
	}
}

// checkForUpdates checks for new versions and logs a warning if an update is available
func checkForUpdates() {
	info := version.CheckLatestVersion()

	if info.Error != nil {
		// Silently ignore errors during version check
		log.Printf("Version check: %v", info.Error)
		return
	}

	if info.UpdateAvailable {
		log.Printf("⚠️  UPDATE AVAILABLE: Container Census %s → %s", info.CurrentVersion, info.LatestVersion)
		log.Printf("   Download: %s", info.ReleaseURL)
	}
}

// runDailyVersionCheck performs version checks once per day
func runDailyVersionCheck(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkForUpdates()
		}
	}
}

// runDailyDatabaseCleanup performs database cleanup of redundant scans once per day
func runDailyDatabaseCleanup(ctx context.Context, db *storage.DB) {
	// Run first cleanup after 1 hour (let system stabilize)
	time.Sleep(1 * time.Hour)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Cleanup scans older than 7 days
	cleanupOlderThan := 7

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("Starting database cleanup (removing redundant scans older than %d days)...", cleanupOlderThan)
			deleted, err := db.CleanupRedundantScans(cleanupOlderThan)
			if err != nil {
				log.Printf("Database cleanup failed: %v", err)
			} else {
				log.Printf("Database cleanup completed: removed %d redundant scan records", deleted)
			}
		}
	}
}

// runHourlyStatsAggregation performs stats aggregation every hour
// Converts granular stats older than 1 hour into hourly aggregates to save space
func runHourlyStatsAggregation(ctx context.Context, db *storage.DB) {
	// Run first aggregation after 1 hour (let system collect some data first)
	time.Sleep(1 * time.Hour)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Starting stats aggregation (converting granular data older than 1 hour to hourly aggregates)...")
			aggregated, err := db.AggregateOldStats()
			if err != nil {
				log.Printf("Stats aggregation failed: %v", err)
			} else if aggregated > 0 {
				log.Printf("Stats aggregation completed: created/updated %d hourly aggregate records", aggregated)
			}
		}
	}
}

// runHourlyNotificationCleanup performs notification log cleanup every hour
// Removes old notifications based on 7-day retention and 100-notification limit
func runHourlyNotificationCleanup(ctx context.Context, db *storage.DB) {
	// Run first cleanup after 1 hour
	time.Sleep(1 * time.Hour)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := db.CleanupOldNotifications(); err != nil {
				log.Printf("Notification cleanup failed: %v", err)
			}
		}
	}
}

// getEnvInt gets an integer from environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		var result int
		if _, err := fmt.Sscanf(val, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
