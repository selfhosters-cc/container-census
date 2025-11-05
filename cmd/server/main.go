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
	"github.com/container-census/container-census/internal/migration"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/notifications"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/telemetry"
	"github.com/container-census/container-census/internal/version"
	"github.com/container-census/container-census/internal/vulnerability"
)

// Global scan interval that can be updated dynamically
var (
	scanIntervalMu     sync.RWMutex
	scanIntervalValue  int
	scanIntervalChange = make(chan int, 1)
)

// Global references for scanner integration
var (
	notificationServiceGlobal       *notifications.NotificationService
	vulnerabilitySchedulerGlobal    *vulnerability.Scheduler
)

// serviceRefs holds references to services that need hot-reload
type serviceRefs struct {
	db                   *storage.DB
	scanner              *scanner.Scanner
	telemetryScheduler   *telemetry.Scheduler
	telemetryCtx         context.Context
	telemetryCancel      context.CancelFunc
	notificationService  *notifications.NotificationService
	apiServer            *api.Server
	mu                   sync.RWMutex
}

var services = &serviceRefs{}

// reloadSettings reloads settings from database and applies them to running services
func reloadSettings() error {
	services.mu.Lock()
	defer services.mu.Unlock()

	// Load settings from DB
	settings, err := services.db.LoadSystemSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	log.Println("Reloading system settings...")

	// Update scan interval
	setScanInterval(settings.Scanner.IntervalSeconds)
	log.Printf("✓ Scan interval updated to %d seconds", settings.Scanner.IntervalSeconds)

	// Restart telemetry scheduler if it exists and settings changed
	if services.telemetryScheduler != nil && services.telemetryCancel != nil {
		// Cancel existing scheduler
		services.telemetryCancel()

		// Load endpoints from database
		endpoints, err := services.db.GetTelemetryEndpoints()
		if err != nil {
			log.Printf("Warning: Failed to load telemetry endpoints: %v", err)
		} else {
			// Create new telemetry config
			telemetryConfig := models.TelemetryConfig{
				IntervalHours: settings.Telemetry.IntervalHours,
				Endpoints:     endpoints,
			}

			// Create new scheduler
			newScheduler, err := telemetry.NewScheduler(services.db, services.scanner, telemetryConfig, settings.Scanner.IntervalSeconds)
			if err != nil {
				log.Printf("Warning: Failed to create new telemetry scheduler: %v", err)
			} else {
				// Create new context
				newCtx, newCancel := context.WithCancel(context.Background())
				services.telemetryCtx = newCtx
				services.telemetryCancel = newCancel
				services.telemetryScheduler = newScheduler

				// Start new scheduler
				go newScheduler.Start(newCtx)
				log.Printf("✓ Telemetry scheduler restarted (interval: %d hours)", settings.Telemetry.IntervalHours)
			}
		}
	}

	// Update notification service settings if it exists
	if services.notificationService != nil {
		// Note: Notification service settings are loaded from environment variables on startup
		// and cannot be hot-reloaded. This would require refactoring the notification service
		// to support UpdateSettings() method. For now, log a warning.
		log.Printf("Note: Notification settings require restart to take effect")
	}

	log.Println("✅ Settings reloaded successfully")
	return nil
}

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

	// Get database path from environment or use default
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/census.db"
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Initialize database
	db, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized: %s", dbPath)

	// Store database reference for hot-reload
	services.db = db

	// Auto-import YAML config on first run (if config file exists)
	if db.IsFirstRun() {
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "./config/config.yaml"
		}
		log.Println("First run detected - attempting to migrate configuration from YAML to database...")
		if err := migration.ImportYAMLConfig(configPath, db); err != nil {
			log.Printf("Warning: Configuration migration failed: %v", err)
			log.Println("Continuing with default settings...")
		}
	}

	// Load settings from database (will use defaults if migration failed)
	settings, err := db.LoadSystemSettings()
	if err != nil {
		log.Fatalf("Failed to load system settings: %v", err)
	}
	log.Printf("System settings loaded from database (scanner interval: %ds, telemetry interval: %dh)",
		settings.Scanner.IntervalSeconds, settings.Telemetry.IntervalHours)

	// Initialize default notification channels and rules
	if err := db.InitializeDefaultNotifications(); err != nil {
		log.Printf("Warning: Failed to initialize default notifications: %v", err)
	}

	// Initialize default telemetry endpoints (community collector)
	if err := db.InitializeDefaultTelemetryEndpoints(); err != nil {
		log.Printf("Warning: Failed to initialize default telemetry endpoints: %v", err)
	}

	// Initialize scanner (using database settings)
	scan := scanner.New(settings.Scanner.TimeoutSeconds)
	log.Println("Scanner initialized")

	// Store scanner reference for hot-reload
	services.scanner = scan

	// Initialize scan interval (from database settings)
	setScanInterval(settings.Scanner.IntervalSeconds)
	log.Printf("Scan interval set to %d seconds", settings.Scanner.IntervalSeconds)

	// Get authentication config from environment variables
	authConfig := getAuthConfigFromEnv()
	if authConfig.Enabled {
		log.Printf("Authentication enabled for user: %s", authConfig.Username)
	} else {
		log.Println("Authentication disabled - UI and API are publicly accessible")
	}

	// Get server host and port from environment variables
	serverHost := os.Getenv("SERVER_HOST")
	if serverHost == "" {
		serverHost = "0.0.0.0"
	}
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	apiServer := api.New(db, scan, settings.Scanner.IntervalSeconds, authConfig)
	apiServer.SetScanIntervalCallback(setScanInterval) // Allow API to update scan interval dynamically
	apiServer.SetReloadSettingsCallback(reloadSettings) // Allow API to trigger hot-reload
	addr := fmt.Sprintf("%s:%s", serverHost, serverPort)

	// Store API server reference for hot-reload
	services.apiServer = apiServer

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

	go runPeriodicScans(ctx, db, scan, settings.Scanner.IntervalSeconds)

	// Start telemetry scheduler if any endpoint is enabled
	endpoints, err := db.GetTelemetryEndpoints()
	if err != nil {
		log.Printf("Warning: Failed to load telemetry endpoints: %v", err)
	}
	enabledCount := 0
	for _, ep := range endpoints {
		if ep.Enabled {
			enabledCount++
		}
	}
	if enabledCount > 0 {
		telemetryConfig := models.TelemetryConfig{
			IntervalHours: settings.Telemetry.IntervalHours,
			Endpoints:     endpoints,
		}
		telemetryScheduler, err := telemetry.NewScheduler(db, scan, telemetryConfig, settings.Scanner.IntervalSeconds)
		if err != nil {
			log.Printf("Warning: Failed to initialize telemetry: %v", err)
		} else {
			telemetryCtx, telemetryCancel := context.WithCancel(ctx)

			// Store references for hot-reload
			services.telemetryScheduler = telemetryScheduler
			services.telemetryCtx = telemetryCtx
			services.telemetryCancel = telemetryCancel

			apiServer.SetTelemetryScheduler(telemetryScheduler, telemetryCtx, telemetryCancel)
			go telemetryScheduler.Start(telemetryCtx)
			log.Printf("Telemetry scheduler started (%d enabled endpoints)", enabledCount)
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

	// Initialize notification system (settings from database, with env var overrides)
	maxNotificationsPerHour := getEnvInt("NOTIFICATION_RATE_LIMIT_MAX", settings.Notification.RateLimitMax)
	batchIntervalSeconds := getEnvInt("NOTIFICATION_RATE_LIMIT_BATCH_INTERVAL", settings.Notification.RateLimitBatchInterval)
	notificationService := notifications.NewNotificationService(db, maxNotificationsPerHour, time.Duration(batchIntervalSeconds)*time.Second)
	notificationServiceGlobal = notificationService // Set global reference for scanner
	services.notificationService = notificationService // Store for hot-reload
	log.Printf("Notification service initialized (rate limit: %d/hour, batch interval: %ds)", maxNotificationsPerHour, batchIntervalSeconds)

	// Pass notification service to API server
	apiServer.SetNotificationService(notificationService)

	// Start baseline stats collector
	baselineCollector := notifications.NewBaselineCollector(db)
	go baselineCollector.StartPeriodicUpdates(ctx)
	log.Println("Baseline stats collector started")

	// Start hourly notification cleanup
	go runHourlyNotificationCleanup(ctx, db)

	// Initialize vulnerability scanner (check database settings only)
	vulnConfig, err := db.LoadVulnerabilitySettings()
	if err != nil {
		log.Printf("Failed to load vulnerability settings from database: %v", err)
		log.Println("Vulnerability scanning disabled")
	} else if vulnConfig.GetEnabled() {
		log.Printf("Loaded vulnerability settings from database (cache_dir: %s)", vulnConfig.GetCacheDir())

		vulnScanner := vulnerability.NewScanner(vulnConfig, db)
		vulnScheduler := vulnerability.NewScheduler(vulnScanner, vulnConfig)
		vulnScheduler.Start()
		log.Printf("Vulnerability scanner initialized (%d workers, auto-scan: %v)", vulnConfig.GetWorkerPoolSize(), vulnConfig.GetAutoScanNewImages())

		// Pass to API server
		apiServer.SetVulnerabilityScanner(vulnScanner, vulnScheduler)

		// Set global reference for container scanner integration
		vulnerabilitySchedulerGlobal = vulnScheduler

		// Start daily Trivy DB update
		go runDailyTrivyDBUpdate(ctx, vulnScanner, vulnConfig)

		// Start daily vulnerability cleanup
		go runDailyVulnerabilityCleanup(ctx, db, vulnConfig)
	} else {
		log.Println("Vulnerability scanning disabled")
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

// getAuthConfigFromEnv loads authentication config from environment variables
func getAuthConfigFromEnv() auth.Config {
	authEnabled := os.Getenv("AUTH_ENABLED")
	authUsername := os.Getenv("AUTH_USERNAME")
	authPassword := os.Getenv("AUTH_PASSWORD")

	enabled := false
	if authEnabled == "true" || authEnabled == "1" || authEnabled == "yes" {
		enabled = true
	}

	return auth.Config{
		Enabled:  enabled,
		Username: authUsername,
		Password: authPassword,
	}
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

			// Queue unique images for vulnerability scanning
			if vulnerabilitySchedulerGlobal != nil {
				queueImagesForScanning(containers, host.ID, db)
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

// queueImagesForScanning queues unique images found in containers for vulnerability scanning
func queueImagesForScanning(containers []models.Container, hostID int64, db *storage.DB) {
	// Track unique images
	seenImages := make(map[string]string) // imageID -> imageName

	for _, container := range containers {
		if container.ImageID == "" {
			continue
		}

		// Only queue each image once
		if _, seen := seenImages[container.ImageID]; !seen {
			seenImages[container.ImageID] = container.Image

			// Update image-to-container mapping
			if err := db.UpdateImageContainer(container.ImageID, container.ID, int(hostID)); err != nil {
				log.Printf("Warning: Failed to update image-container mapping: %v", err)
			}

			// Queue for scanning (non-blocking)
			// Note: QueueScan internally checks NeedsScan() and returns nil if already scanned recently
			if err := vulnerabilitySchedulerGlobal.QueueScan(container.ImageID, container.Image, 0); err != nil {
				log.Printf("Warning: Failed to queue image for scanning: %v", err)
			}
		}
	}

	// Note: Not logging queue count here because QueueScan silently skips images that
	// don't need scanning (already scanned within cache TTL). Check queue status on
	// Security tab to see actual scanning activity.
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

// runDailyTrivyDBUpdate updates the Trivy database daily
func runDailyTrivyDBUpdate(ctx context.Context, scanner *vulnerability.Scanner, config *vulnerability.Config) {
	// Calculate time until next 2 AM
	now := time.Now()
	next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if now.After(next2AM) {
		next2AM = next2AM.Add(24 * time.Hour)
	}
	initialDelay := time.Until(next2AM)

	log.Printf("Trivy database will be updated at %s", next2AM.Format("2006-01-02 15:04:05"))
	time.Sleep(initialDelay)

	// Run first update
	log.Println("Running Trivy database update...")
	updateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	if err := scanner.UpdateTrivyDB(updateCtx); err != nil {
		log.Printf("Trivy database update failed: %v", err)
	} else {
		log.Println("Trivy database updated successfully")
	}
	cancel()

	// Run daily at 2 AM
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Running scheduled Trivy database update...")
			updateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			if err := scanner.UpdateTrivyDB(updateCtx); err != nil {
				log.Printf("Trivy database update failed: %v", err)
			} else {
				log.Println("Trivy database updated successfully")
			}
			cancel()
		}
	}
}

// runDailyVulnerabilityCleanup performs vulnerability data cleanup daily
func runDailyVulnerabilityCleanup(ctx context.Context, db *storage.DB, config *vulnerability.Config) {
	// Calculate time until next 3 AM
	now := time.Now()
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if now.After(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}
	initialDelay := time.Until(next3AM)

	log.Printf("Vulnerability cleanup will run at %s", next3AM.Format("2006-01-02 15:04:05"))
	time.Sleep(initialDelay)

	// Run first cleanup
	retentionDays := config.GetRetentionDays()
	detailedRetentionDays := config.GetDetailedRetentionDays()
	if err := db.CleanupOldVulnerabilityData(retentionDays, detailedRetentionDays); err != nil {
		log.Printf("Vulnerability cleanup failed: %v", err)
	} else {
		log.Println("Vulnerability cleanup completed successfully")
	}

	// Run daily at 3 AM
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Running scheduled vulnerability cleanup...")
			// Re-read retention values in case they were updated
			retentionDays := config.GetRetentionDays()
			detailedRetentionDays := config.GetDetailedRetentionDays()
			if err := db.CleanupOldVulnerabilityData(retentionDays, detailedRetentionDays); err != nil {
				log.Printf("Vulnerability cleanup failed: %v", err)
			} else {
				log.Println("Vulnerability cleanup completed successfully")
			}
		}
	}
}
