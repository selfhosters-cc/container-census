package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/version"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq" // PostgreSQL driver
)

type Config struct {
	DatabaseURL  string
	Port         int
	AuthEnabled  bool
	AuthUsername string
	AuthPassword string
	StatsAPIKey  string // API key for stats endpoints
}

type Server struct {
	db     *sql.DB
	router *mux.Router
	config Config
}

type SubmissionEvent struct {
	ID             int       `json:"id"`
	InstallationID string    `json:"installation_id"`
	EventType      string    `json:"event_type"` // "new" or "update"
	Timestamp      time.Time `json:"timestamp"`
	Containers     int       `json:"containers"`
	Hosts          int       `json:"hosts"`
}

func main() {
	log.Printf("Starting Telemetry Collector Service v%s...", version.Get())

	// Load configuration from environment
	config := Config{
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable"),
		Port:         getEnvInt("PORT", 8081),
		AuthEnabled:  getEnv("COLLECTOR_AUTH_ENABLED", "") == "true",
		AuthUsername: getEnv("COLLECTOR_AUTH_USERNAME", ""),
		AuthPassword: getEnv("COLLECTOR_AUTH_PASSWORD", ""),
		StatsAPIKey:  getEnv("STATS_API_KEY", ""),
	}

	if config.AuthEnabled {
		log.Printf("Authentication enabled for telemetry collector dashboard (user: %s)", config.AuthUsername)
	} else {
		log.Println("Authentication disabled - telemetry collector dashboard is publicly accessible")
	}

	if config.StatsAPIKey != "" {
		log.Println("API key authentication enabled for stats endpoints")
	} else {
		log.Println("Warning: No STATS_API_KEY set - stats API endpoints are publicly accessible")
	}

	// Connect to database
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connected successfully")

	// Initialize schema
	if err := initSchema(db); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}
	log.Println("Database schema initialized")

	// Create server
	server := &Server{
		db:     db,
		router: mux.NewRouter(),
		config: config,
	}

	server.setupRoutes()

	// HTTP server
	addr := fmt.Sprintf(":%d", config.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Check for updates on startup
	go checkForUpdates()

	// Create context for background tasks
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Start daily version check
	go runDailyVersionCheck(bgCtx)

	// Start server
	go func() {
		log.Printf("Telemetry collector listening on http://0.0.0.0%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	bgCancel() // Cancel background tasks

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

func (s *Server) setupRoutes() {
	// Health endpoint - always public (for monitoring)
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Ingest endpoint - always public (anonymous telemetry submission)
	s.router.HandleFunc("/api/ingest", s.handleIngest).Methods("POST")

	// Stats API - protected by API key (read-only analytics data)
	s.router.HandleFunc("/api/stats/top-images", s.apiKeyMiddleware(s.handleTopImages)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/image-details", s.apiKeyMiddleware(s.handleImageDetails)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/growth", s.apiKeyMiddleware(s.handleGrowth)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/installations", s.apiKeyMiddleware(s.handleInstallations)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/summary", s.apiKeyMiddleware(s.handleSummary)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/registries", s.apiKeyMiddleware(s.handleRegistries)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/versions", s.apiKeyMiddleware(s.handleVersions)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/activity-heatmap", s.apiKeyMiddleware(s.handleActivityHeatmap)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/scan-intervals", s.apiKeyMiddleware(s.handleScanIntervals)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/geography", s.apiKeyMiddleware(s.handleGeography)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/connection-metrics", s.apiKeyMiddleware(s.handleConnectionMetrics)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/recent-events", s.apiKeyMiddleware(s.handleRecentEvents)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/database-view", s.apiKeyMiddleware(s.handleDatabaseView)).Methods("GET", "OPTIONS")

	// Static files for analytics dashboard - protected if auth is enabled
	if s.config.AuthEnabled {
		authMiddleware := s.basicAuthMiddleware()
		s.router.PathPrefix("/").Handler(authMiddleware(http.FileServer(http.Dir("./web/analytics"))))
	} else {
		s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/analytics")))
	}
}

// basicAuthMiddleware creates HTTP Basic Auth middleware
func (s *Server) basicAuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()

			if !ok || username != s.config.AuthUsername || password != s.config.AuthPassword {
				w.Header().Set("WWW-Authenticate", `Basic realm="Telemetry Collector", charset="UTF-8"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// apiKeyMiddleware creates API key authentication middleware
func (s *Server) apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no API key is configured, allow all requests
		if s.config.StatsAPIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for API key in X-API-Key header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Also check Authorization header with Bearer token
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				apiKey = authHeader[7:]
			}
		}

		if apiKey != s.config.StatsAPIKey {
			respondError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}

		// Add CORS headers for cross-origin requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization, Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// Health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	response := map[string]interface{}{
		"status":  "healthy",
		"version": version.Get(),
		"time":    time.Now().Format(time.RFC3339),
	}

	// Add update information if available
	updateInfo := version.GetUpdateInfo()
	if updateInfo != nil && updateInfo.Error == nil {
		response["latest_version"] = updateInfo.LatestVersion
		response["update_available"] = updateInfo.UpdateAvailable
		if updateInfo.UpdateAvailable {
			response["release_url"] = updateInfo.ReleaseURL
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// Ingest telemetry data
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var report models.TelemetryReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate
	if report.InstallationID == "" {
		respondError(w, http.StatusBadRequest, "Missing installation_id")
		return
	}

	// Save to database
	if err := s.saveTelemetry(report); err != nil {
		log.Printf("Failed to save telemetry: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to save telemetry")
		return
	}

	log.Printf("Received telemetry from installation %s: %d containers, %d images",
		report.InstallationID, report.TotalContainers, len(report.ImageStats))

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": "Telemetry received",
	})
}

// Save telemetry to database
func (s *Server) saveTelemetry(report models.TelemetryReport) error {
	eventType := "new" // Will be set to "update" if we UPDATE existing record
	// Serialize JSON fields
	imageStatsJSON, err := json.Marshal(report.ImageStats)
	if err != nil {
		return fmt.Errorf("failed to marshal image stats: %w", err)
	}

	agentVersionsJSON, err := json.Marshal(report.AgentVersions)
	if err != nil {
		return fmt.Errorf("failed to marshal agent versions: %w", err)
	}

	// Start a transaction to ensure atomicity
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if a report exists for this installation in the current week
	// Week is defined as the 7-day period starting from the most recent report
	checkQuery := `
		SELECT id, timestamp
		FROM telemetry_reports
		WHERE installation_id = $1
		  AND timestamp >= NOW() - INTERVAL '7 days'
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var existingID int
	var existingTimestamp time.Time
	err = tx.QueryRow(checkQuery, report.InstallationID).Scan(&existingID, &existingTimestamp)

	if err == nil {
		// Record exists within the last 7 days - UPDATE it
		eventType = "update"
		updateQuery := `
			UPDATE telemetry_reports
			SET version = $2,
			    timestamp = $3,
			    host_count = $4,
			    agent_count = $5,
			    total_containers = $6,
			    scan_interval = $7,
			    image_stats = $8,
			    agent_versions = $9,
			    containers_running = $10,
			    containers_stopped = $11,
			    containers_paused = $12,
			    containers_other = $13,
			    avg_cpu_percent = $14,
			    avg_memory_bytes = $15,
			    total_memory_limit = $16,
			    avg_restarts = $17,
			    high_restart_containers = $18,
			    total_image_size = $19,
			    unique_images = $20,
			    timezone = $21,
			    compose_project_count = $22,
			    containers_in_compose = $23,
			    network_count = $24,
			    custom_network_count = $25,
			    shared_volume_count = $26,
			    containers_with_deps = $27,
			    total_dependencies = $28,
			    avg_connections_per_container = $29
			WHERE id = $1
		`
		_, err = tx.Exec(updateQuery,
			existingID,
			report.Version,
			report.Timestamp,
			report.HostCount,
			report.AgentCount,
			report.TotalContainers,
			report.ScanInterval,
			string(imageStatsJSON),
			string(agentVersionsJSON),
			report.ContainersRunning,
			report.ContainersStopped,
			report.ContainersPaused,
			report.ContainersOther,
			report.AvgCPUPercent,
			report.AvgMemoryBytes,
			report.TotalMemoryLimit,
			report.AvgRestarts,
			report.HighRestartContainers,
			report.TotalImageSize,
			report.UniqueImages,
			report.Timezone,
			report.ComposeProjectCount,
			report.ContainersInCompose,
			report.NetworkCount,
			report.CustomNetworkCount,
			report.SharedVolumeCount,
			report.ContainersWithDeps,
			report.TotalDependencies,
			report.AvgConnectionsPerContainer,
		)
		if err != nil {
			return fmt.Errorf("failed to update telemetry: %w", err)
		}

		// Delete old image stats for this installation from the current week
		deleteImagesQuery := `
			DELETE FROM image_stats
			WHERE installation_id = $1
			  AND timestamp >= $2
		`
		_, err = tx.Exec(deleteImagesQuery, report.InstallationID, existingTimestamp)
		if err != nil {
			log.Printf("Warning: Failed to delete old image stats: %v", err)
		}

		log.Printf("Updated existing telemetry report for installation %s (within 7-day window)", report.InstallationID)
	} else {
		// No record in the last 7 days - INSERT new one
		insertQuery := `
			INSERT INTO telemetry_reports (
				installation_id, version, timestamp, host_count, agent_count,
				total_containers, scan_interval, image_stats, agent_versions,
				containers_running, containers_stopped, containers_paused, containers_other,
				avg_cpu_percent, avg_memory_bytes, total_memory_limit,
				avg_restarts, high_restart_containers,
				total_image_size, unique_images, timezone,
				compose_project_count, containers_in_compose, network_count, custom_network_count,
				shared_volume_count, containers_with_deps, total_dependencies, avg_connections_per_container
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		`
		_, err = tx.Exec(insertQuery,
			report.InstallationID,
			report.Version,
			report.Timestamp,
			report.HostCount,
			report.AgentCount,
			report.TotalContainers,
			report.ScanInterval,
			string(imageStatsJSON),
			string(agentVersionsJSON),
			report.ContainersRunning,
			report.ContainersStopped,
			report.ContainersPaused,
			report.ContainersOther,
			report.AvgCPUPercent,
			report.AvgMemoryBytes,
			report.TotalMemoryLimit,
			report.AvgRestarts,
			report.HighRestartContainers,
			report.TotalImageSize,
			report.UniqueImages,
			report.Timezone,
			report.ComposeProjectCount,
			report.ContainersInCompose,
			report.NetworkCount,
			report.CustomNetworkCount,
			report.SharedVolumeCount,
			report.ContainersWithDeps,
			report.TotalDependencies,
			report.AvgConnectionsPerContainer,
		)
		if err != nil {
			return fmt.Errorf("failed to insert telemetry: %w", err)
		}

		log.Printf("Inserted new telemetry report for installation %s", report.InstallationID)
	}

	// Insert fresh image stats with ORIGINAL names (keep registry prefix for registry detection)
	// Normalization is applied during queries for grouping, not storage
	for _, imageStat := range report.ImageStats {
		insertImageQuery := `
			INSERT INTO image_stats (installation_id, timestamp, image, count, size_bytes)
			VALUES ($1, $2, $3, $4, $5)
		`
		// Store original image name with registry prefix intact
		_, err := tx.Exec(insertImageQuery, report.InstallationID, report.Timestamp, imageStat.Image, imageStat.Count, imageStat.SizeBytes)
		if err != nil {
			log.Printf("Warning: Failed to insert image stat: %v", err)
		}
	}

	// Log submission event for live tracking
	eventQuery := `
		INSERT INTO submission_events (installation_id, event_type, timestamp, containers, hosts)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(eventQuery, report.InstallationID, eventType, report.Timestamp, report.TotalContainers, report.HostCount)
	if err != nil {
		log.Printf("Warning: Failed to log submission event: %v", err)
		// Don't fail the whole transaction for this
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Get top images
func (s *Server) handleTopImages(w http.ResponseWriter, r *http.Request) {
	limit := getQueryInt(r, "limit", 20)
	days := getQueryInt(r, "days", 30)

	since := time.Now().AddDate(0, 0, -days)

	// First, get total unique installations for percentage calculation
	var totalInstallations int
	totalQuery := `
		SELECT COUNT(DISTINCT installation_id)
		FROM telemetry_reports
		WHERE timestamp >= $1
	`
	err := s.db.QueryRow(totalQuery, since).Scan(&totalInstallations)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get total installations: "+err.Error())
		return
	}

	// Deduplicate by using only the most recent image stats per installation
	// This prevents counting the same installation multiple times
	// Apply normalization at query time to handle both old and new data
	// Include installation count and adoption percentage
	query := `
		SELECT
			normalized_image,
			SUM(count) as total_count,
			COUNT(DISTINCT installation_id) as installation_count
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				installation_id,
				-- Normalize image names by removing registry prefixes
				REGEXP_REPLACE(
					REGEXP_REPLACE(
						REGEXP_REPLACE(
							REGEXP_REPLACE(
								REGEXP_REPLACE(
									REGEXP_REPLACE(
										REGEXP_REPLACE(image, '^ghcr\.io/', ''),
									'^docker\.io/', ''),
								'^hub\.docker\.com/', ''),
							'^registry\.hub\.docker\.com/', ''),
						'^quay\.io/', ''),
					'^gcr\.io/', ''),
				'^mcr\.microsoft\.com/', '') as normalized_image,
				count
			FROM image_stats
			WHERE timestamp >= $1
			ORDER BY installation_id, image, timestamp DESC
		) latest_stats
		GROUP BY normalized_image
		ORDER BY total_count DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, since, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type ImageCount struct {
		Image              string  `json:"image"`
		Count              int     `json:"count"`
		InstallationCount  int     `json:"installation_count"`
		AdoptionPercentage float64 `json:"adoption_percentage"`
	}

	var results []ImageCount
	for rows.Next() {
		var ic ImageCount
		if err := rows.Scan(&ic.Image, &ic.Count, &ic.InstallationCount); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		// Calculate adoption percentage
		if totalInstallations > 0 {
			ic.AdoptionPercentage = float64(ic.InstallationCount) / float64(totalInstallations) * 100
			// Round to 1 decimal place
			ic.AdoptionPercentage = float64(int(ic.AdoptionPercentage*10)) / 10
		}
		results = append(results, ic)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get detailed image data with size, count, registry info
func (s *Server) handleImageDetails(w http.ResponseWriter, r *http.Request) {
	limit := getQueryInt(r, "limit", 100)
	offset := getQueryInt(r, "offset", 0)
	days := getQueryInt(r, "days", 30)
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort_by")    // name, count, size
	sortOrder := r.URL.Query().Get("sort_order") // asc, desc

	// Default sorting
	if sortBy == "" {
		sortBy = "count"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Validate sortBy to prevent SQL injection
	validSortColumns := map[string]string{
		"name":  "normalized_image",
		"count": "total_count",
		"size":  "total_size",
	}
	sortColumn, ok := validSortColumns[sortBy]
	if !ok {
		sortColumn = "total_count"
	}

	// Validate sortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	since := time.Now().AddDate(0, 0, -days)

	// Build query with optional search filter
	searchFilter := ""
	args := []interface{}{since}
	argNum := 2
	if search != "" {
		searchFilter = fmt.Sprintf("AND normalized_image ILIKE $%d", argNum)
		args = append(args, "%"+search+"%")
		argNum++
	}

	// Main query with normalization, deduplication, and registry detection
	query := fmt.Sprintf(`
		SELECT
			normalized_image,
			SUM(count) as total_count,
			-- Use MAX to pick the most specific registry (prefer explicit registry over Docker Hub)
			MAX(CASE
				WHEN registry != 'Docker Hub' THEN registry
				ELSE registry
			END) as registry,
			COUNT(DISTINCT installation_id) as installation_count
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				installation_id,
				-- Normalize image names by removing registry prefixes
				REGEXP_REPLACE(
					REGEXP_REPLACE(
						REGEXP_REPLACE(
							REGEXP_REPLACE(
								REGEXP_REPLACE(
									REGEXP_REPLACE(
										REGEXP_REPLACE(image, '^ghcr\.io/', ''),
									'^docker\.io/', ''),
								'^hub\.docker\.com/', ''),
							'^registry\.hub\.docker\.com/', ''),
						'^quay\.io/', ''),
					'^gcr\.io/', ''),
				'^mcr\.microsoft\.com/', '') as normalized_image,
				-- Detect registry from original image name
				CASE
					WHEN image ~ '^ghcr\.io/' THEN 'ghcr.io'
					WHEN image ~ '^quay\.io/' THEN 'quay.io'
					WHEN image ~ '^gcr\.io/' THEN 'gcr.io'
					WHEN image ~ '^mcr\.microsoft\.com/' THEN 'mcr.microsoft.com'
					ELSE 'Docker Hub'
				END as registry,
				count
			FROM image_stats
			WHERE timestamp >= $1
			ORDER BY installation_id, image, timestamp DESC
		) latest_stats
		WHERE 1=1 %s
		GROUP BY normalized_image
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, searchFilter, sortColumn, sortOrder, argNum, argNum+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type ImageDetail struct {
		Image             string `json:"image"`
		Count             int    `json:"count"`
		Registry          string `json:"registry"`
		InstallationCount int    `json:"installation_count"`
	}

	var results []ImageDetail
	for rows.Next() {
		var id ImageDetail
		if err := rows.Scan(&id.Image, &id.Count, &id.Registry, &id.InstallationCount); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, id)
	}

	// Get total count for pagination metadata
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT normalized_image)
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				installation_id,
				REGEXP_REPLACE(
					REGEXP_REPLACE(
						REGEXP_REPLACE(
							REGEXP_REPLACE(
								REGEXP_REPLACE(
									REGEXP_REPLACE(
										REGEXP_REPLACE(image, '^ghcr\.io/', ''),
									'^docker\.io/', ''),
								'^hub\.docker\.com/', ''),
							'^registry\.hub\.docker\.com/', ''),
						'^quay\.io/', ''),
					'^gcr\.io/', ''),
				'^mcr\.microsoft\.com/', '') as normalized_image
			FROM image_stats
			WHERE timestamp >= $1
			ORDER BY installation_id, image, timestamp DESC
		) latest_stats
		WHERE 1=1 %s
	`, searchFilter)

	var totalCount int
	countArgs := []interface{}{since}
	if search != "" {
		countArgs = append(countArgs, "%"+search+"%")
	}

	err = s.db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		log.Printf("Count query error: %v", err)
		totalCount = len(results) // Fallback
	}

	// Return results with pagination metadata
	response := map[string]interface{}{
		"images": results,
		"pagination": map[string]interface{}{
			"total":  totalCount,
			"limit":  limit,
			"offset": offset,
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// Get growth metrics
func (s *Server) handleGrowth(w http.ResponseWriter, r *http.Request) {
	days := getQueryInt(r, "days", 90)
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT DATE(timestamp) as date, COUNT(DISTINCT installation_id) as installations,
		       AVG(total_containers) as avg_containers
		FROM telemetry_reports
		WHERE timestamp >= $1
		GROUP BY DATE(timestamp)
		ORDER BY date ASC
	`

	rows, err := s.db.Query(query, since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type Growth struct {
		Date           string  `json:"date"`
		Installations  int     `json:"installations"`
		AvgContainers  float64 `json:"avg_containers"`
	}

	var results []Growth
	for rows.Next() {
		var g Growth
		if err := rows.Scan(&g.Date, &g.Installations, &g.AvgContainers); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, g)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get installation stats
func (s *Server) handleInstallations(w http.ResponseWriter, r *http.Request) {
	days := getQueryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT COUNT(DISTINCT installation_id) as total_installations
		FROM telemetry_reports
		WHERE timestamp >= $1
	`

	var count int
	err := s.db.QueryRow(query, since).Scan(&count)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]int{
		"total_installations": count,
		"period_days":         days,
	})
}

// Get summary stats
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	// Get installation stats
	installQuery := `
		SELECT
			COUNT(DISTINCT installation_id) as installations,
			SUM(total_containers) as total_containers,
			SUM(host_count) as total_hosts,
			SUM(agent_count) as total_agents
		FROM (
			SELECT DISTINCT ON (installation_id)
				installation_id, total_containers, host_count, agent_count
			FROM telemetry_reports
			ORDER BY installation_id, timestamp DESC
		) recent_installations
	`

	// Get unique images count separately using deduplication
	imagesQuery := `
		SELECT COUNT(DISTINCT image) as unique_images
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				image
			FROM image_stats
			WHERE timestamp >= NOW() - INTERVAL '30 days'
			ORDER BY installation_id, image, timestamp DESC
		) latest_images
	`

	// Get total submissions count
	submissionsQuery := `
		SELECT COUNT(*) as total_submissions
		FROM telemetry_reports
	`

	type Summary struct {
		Installations          int     `json:"installations"`
		TotalSubmissions       int     `json:"total_submissions"`
		TotalContainers        int     `json:"total_containers"`
		AvgContainersPerInstall float64 `json:"avg_containers_per_install"`
		TotalHosts             int     `json:"total_hosts"`
		TotalAgents            int     `json:"total_agents"`
		UniqueImages           int     `json:"unique_images"`
	}

	var summary Summary

	// Get installation stats
	err := s.db.QueryRow(installQuery).Scan(
		&summary.Installations,
		&summary.TotalContainers,
		&summary.TotalHosts,
		&summary.TotalAgents,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query installation stats: "+err.Error())
		return
	}

	// Calculate average containers per installation
	if summary.Installations > 0 {
		summary.AvgContainersPerInstall = float64(summary.TotalContainers) / float64(summary.Installations)
		// Round to 1 decimal place
		summary.AvgContainersPerInstall = float64(int(summary.AvgContainersPerInstall*10)) / 10
	}

	// Get unique images count
	err = s.db.QueryRow(imagesQuery).Scan(&summary.UniqueImages)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query unique images: "+err.Error())
		return
	}

	// Get total submissions
	err = s.db.QueryRow(submissionsQuery).Scan(&summary.TotalSubmissions)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query total submissions: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// Get registry distribution stats
func (s *Server) handleRegistries(w http.ResponseWriter, r *http.Request) {
	days := getQueryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT
			CASE
				WHEN image LIKE 'ghcr.io/%' THEN 'GitHub Container Registry'
				WHEN image LIKE 'quay.io/%' THEN 'Quay.io'
				WHEN image LIKE 'gcr.io/%' THEN 'Google Container Registry'
				WHEN image LIKE 'mcr.microsoft.com/%' THEN 'Microsoft Container Registry'
				WHEN image LIKE '%/%.%/%' THEN 'Other Private Registry'
				ELSE 'Docker Hub'
			END as registry,
			SUM(count) as total_count
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				image,
				count
			FROM image_stats
			WHERE timestamp >= $1
			ORDER BY installation_id, image, timestamp DESC
		) latest_stats
		GROUP BY registry
		ORDER BY total_count DESC
	`

	rows, err := s.db.Query(query, since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type RegistryCount struct {
		Registry string `json:"registry"`
		Count    int    `json:"count"`
	}

	var results []RegistryCount
	for rows.Next() {
		var rc RegistryCount
		if err := rows.Scan(&rc.Registry, &rc.Count); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, rc)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get agent version distribution
func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT version, COUNT(DISTINCT installation_id) as installations
		FROM (
			SELECT DISTINCT ON (installation_id)
				installation_id,
				version
			FROM telemetry_reports
			WHERE timestamp >= NOW() - INTERVAL '30 days'
			ORDER BY installation_id, timestamp DESC
		) latest_reports
		WHERE version IS NOT NULL AND version != ''
		GROUP BY version
		ORDER BY installations DESC
		LIMIT 10
	`

	rows, err := s.db.Query(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type VersionCount struct {
		Version       string `json:"version"`
		Installations int    `json:"installations"`
	}

	var results []VersionCount
	for rows.Next() {
		var vc VersionCount
		if err := rows.Scan(&vc.Version, &vc.Installations); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, vc)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get activity heatmap data (reports by hour of day and day of week)
func (s *Server) handleActivityHeatmap(w http.ResponseWriter, r *http.Request) {
	days := getQueryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT
			EXTRACT(DOW FROM timestamp) as day_of_week,
			EXTRACT(HOUR FROM timestamp) as hour_of_day,
			COUNT(*) as report_count
		FROM telemetry_reports
		WHERE timestamp >= $1
		GROUP BY day_of_week, hour_of_day
		ORDER BY day_of_week, hour_of_day
	`

	rows, err := s.db.Query(query, since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type HeatmapData struct {
		DayOfWeek   int `json:"day_of_week"`   // 0=Sunday, 6=Saturday
		HourOfDay   int `json:"hour_of_day"`   // 0-23
		ReportCount int `json:"report_count"`
	}

	var results []HeatmapData
	for rows.Next() {
		var hd HeatmapData
		if err := rows.Scan(&hd.DayOfWeek, &hd.HourOfDay, &hd.ReportCount); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, hd)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get scan interval distribution
func (s *Server) handleScanIntervals(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			scan_interval,
			COUNT(DISTINCT installation_id) as installations
		FROM (
			SELECT DISTINCT ON (installation_id)
				installation_id,
				scan_interval
			FROM telemetry_reports
			WHERE timestamp >= NOW() - INTERVAL '30 days'
			ORDER BY installation_id, timestamp DESC
		) latest_reports
		WHERE scan_interval > 0
		GROUP BY scan_interval
		ORDER BY installations DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type IntervalCount struct {
		Interval      int `json:"interval"`      // seconds
		Installations int `json:"installations"`
	}

	var results []IntervalCount
	for rows.Next() {
		var ic IntervalCount
		if err := rows.Scan(&ic.Interval, &ic.Installations); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, ic)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get geographic distribution based on timezone data
func (s *Server) handleGeography(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			COALESCE(timezone, 'Unknown') as timezone,
			COUNT(DISTINCT installation_id) as installations
		FROM (
			SELECT DISTINCT ON (installation_id)
				installation_id,
				timezone
			FROM telemetry_reports
			WHERE timestamp >= NOW() - INTERVAL '30 days'
			ORDER BY installation_id, timestamp DESC
		) latest_reports
		GROUP BY timezone
		ORDER BY installations DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	type GeographyData struct {
		Timezone      string `json:"timezone"`
		Installations int    `json:"installations"`
		Region        string `json:"region"` // Derived from timezone
	}

	var results []GeographyData
	for rows.Next() {
		var gd GeographyData
		if err := rows.Scan(&gd.Timezone, &gd.Installations); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		// Derive region from timezone
		gd.Region = getRegionFromTimezone(gd.Timezone)
		results = append(results, gd)
	}

	respondJSON(w, http.StatusOK, results)
}

// Get connection and architecture metrics
func (s *Server) handleConnectionMetrics(w http.ResponseWriter, r *http.Request) {
	days := getQueryInt(r, "days", 30)
	since := time.Now().AddDate(0, 0, -days)

	query := `
		SELECT
			COALESCE(SUM(total_containers), 0) as total_containers,
			COALESCE(SUM(compose_project_count), 0) as total_projects,
			COALESCE(SUM(containers_in_compose), 0) as containers_in_compose,
			COALESCE(SUM(network_count), 0) as network_count,
			COALESCE(SUM(custom_network_count), 0) as custom_network_count,
			COALESCE(SUM(shared_volume_count), 0) as shared_volume_count,
			COALESCE(SUM(containers_with_deps), 0) as containers_with_deps,
			COALESCE(SUM(total_dependencies), 0) as total_dependencies,
			COALESCE(AVG(avg_connections_per_container), 0) as avg_connections_per_container,
			COUNT(DISTINCT installation_id) as installations
		FROM (
			SELECT DISTINCT ON (installation_id)
				installation_id,
				total_containers,
				compose_project_count,
				containers_in_compose,
				network_count,
				custom_network_count,
				shared_volume_count,
				containers_with_deps,
				total_dependencies,
				avg_connections_per_container
			FROM telemetry_reports
			WHERE timestamp >= $1
			ORDER BY installation_id, timestamp DESC
		) latest_reports
	`

	var result struct {
		TotalContainers             int     `json:"total_containers"`
		TotalProjects               int     `json:"compose_project_count"`
		ContainersInCompose         int     `json:"containers_in_compose"`
		ComposePercentage           float64 `json:"compose_percentage"`
		NetworkCount                int     `json:"network_count"`
		CustomNetworkCount          int     `json:"custom_network_count"`
		SharedVolumeCount           int     `json:"shared_volume_count"`
		TotalVolumes                int     `json:"total_volumes"` // Estimated from shared volumes
		ContainersWithDeps          int     `json:"containers_with_deps"`
		TotalDependencies           int     `json:"total_dependencies"`
		AvgConnectionsPerContainer  float64 `json:"avg_connections_per_container"`
		Installations               int     `json:"installations"`
	}

	err := s.db.QueryRow(query, since).Scan(
		&result.TotalContainers,
		&result.TotalProjects,
		&result.ContainersInCompose,
		&result.NetworkCount,
		&result.CustomNetworkCount,
		&result.SharedVolumeCount,
		&result.ContainersWithDeps,
		&result.TotalDependencies,
		&result.AvgConnectionsPerContainer,
		&result.Installations,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}

	// Calculate compose adoption percentage
	if result.TotalContainers > 0 {
		result.ComposePercentage = float64(result.ContainersInCompose) / float64(result.TotalContainers) * 100
		result.ComposePercentage = float64(int(result.ComposePercentage*10)) / 10 // Round to 1 decimal
	}

	// Estimate total volumes (shared volumes + assumed unique volumes per container not sharing)
	// This is a rough estimate since we don't track all volumes, just shared ones
	result.TotalVolumes = result.SharedVolumeCount * 2 // Very rough estimate

	respondJSON(w, http.StatusOK, result)
}

// Helper functions

// getRegionFromTimezone maps timezone to a general region for visualization
func getRegionFromTimezone(tz string) string {
	if tz == "" || tz == "Unknown" || tz == "UTC" {
		return "Unknown"
	}

	// Map common timezone prefixes to regions
	switch {
	case len(tz) >= 7 && tz[:7] == "America":
		return "Americas"
	case len(tz) >= 6 && tz[:6] == "Europe":
		return "Europe"
	case len(tz) >= 4 && tz[:4] == "Asia":
		return "Asia"
	case len(tz) >= 6 && tz[:6] == "Africa":
		return "Africa"
	case len(tz) >= 7 && tz[:7] == "Pacific":
		return "Pacific"
	case len(tz) >= 9 && tz[:9] == "Australia":
		return "Oceania"
	case len(tz) >= 10 && tz[:10] == "Antarctica":
		return "Antarctica"
	default:
		return "Other"
	}
}

// normalizeImageName removes registry prefixes to group images by project/name
// Examples:
//   ghcr.io/project/image:tag -> project/image:tag
//   docker.io/project/image:tag -> project/image:tag
//   hub.docker.com/project/image:tag -> project/image:tag
//   project/image:tag -> project/image:tag
//   image:tag -> image:tag
func normalizeImageName(image string) string {
	// Common registry prefixes to remove
	registries := []string{
		"ghcr.io/",
		"docker.io/",
		"hub.docker.com/",
		"registry.hub.docker.com/",
		"quay.io/",
		"gcr.io/",
		"mcr.microsoft.com/",
	}

	for _, registry := range registries {
		if len(image) > len(registry) && image[:len(registry)] == registry {
			return image[len(registry):]
		}
	}

	return image
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getQueryInt(r *http.Request, key string, defaultValue int) int {
	if value := r.URL.Query().Get(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

// Initialize database schema
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS telemetry_reports (
		id SERIAL PRIMARY KEY,
		installation_id VARCHAR(255) NOT NULL,
		version VARCHAR(50),
		timestamp TIMESTAMPTZ NOT NULL,
		host_count INTEGER NOT NULL DEFAULT 0,
		agent_count INTEGER NOT NULL DEFAULT 0,
		total_containers INTEGER NOT NULL DEFAULT 0,
		scan_interval INTEGER NOT NULL DEFAULT 0,
		image_stats JSONB,
		agent_versions JSONB,
		-- Enhanced metrics fields
		containers_running INTEGER DEFAULT 0,
		containers_stopped INTEGER DEFAULT 0,
		containers_paused INTEGER DEFAULT 0,
		containers_other INTEGER DEFAULT 0,
		avg_cpu_percent REAL DEFAULT 0.0,
		avg_memory_bytes BIGINT DEFAULT 0,
		total_memory_limit BIGINT DEFAULT 0,
		avg_restarts REAL DEFAULT 0.0,
		high_restart_containers INTEGER DEFAULT 0,
		total_image_size BIGINT DEFAULT 0,
		unique_images INTEGER DEFAULT 0,
		timezone VARCHAR(100),
		-- Connection and architecture metrics
		compose_project_count INTEGER DEFAULT 0,
		containers_in_compose INTEGER DEFAULT 0,
		network_count INTEGER DEFAULT 0,
		custom_network_count INTEGER DEFAULT 0,
		shared_volume_count INTEGER DEFAULT 0,
		containers_with_deps INTEGER DEFAULT 0,
		total_dependencies INTEGER DEFAULT 0,
		avg_connections_per_container REAL DEFAULT 0.0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_telemetry_installation_id ON telemetry_reports(installation_id);
	CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON telemetry_reports(timestamp);
	CREATE INDEX IF NOT EXISTS idx_telemetry_installation_timestamp ON telemetry_reports(installation_id, timestamp DESC);

	CREATE TABLE IF NOT EXISTS image_stats (
		id SERIAL PRIMARY KEY,
		installation_id VARCHAR(255) NOT NULL,
		timestamp TIMESTAMPTZ NOT NULL,
		image VARCHAR(500) NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		size_bytes BIGINT DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_image_stats_image ON image_stats(image);
	CREATE INDEX IF NOT EXISTS idx_image_stats_timestamp ON image_stats(timestamp);
	CREATE INDEX IF NOT EXISTS idx_image_stats_installation_id ON image_stats(installation_id);

	CREATE TABLE IF NOT EXISTS submission_events (
		id SERIAL PRIMARY KEY,
		installation_id VARCHAR(255) NOT NULL,
		event_type VARCHAR(10) NOT NULL,
		timestamp TIMESTAMPTZ NOT NULL,
		containers INTEGER NOT NULL DEFAULT 0,
		hosts INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_submission_events_timestamp ON submission_events(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_submission_events_id ON submission_events(id DESC);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add new columns if they don't exist (for existing databases)
	migrations := []string{
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_running INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_stopped INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_paused INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_other INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS avg_cpu_percent REAL DEFAULT 0.0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS avg_memory_bytes BIGINT DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS total_memory_limit BIGINT DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS avg_restarts REAL DEFAULT 0.0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS high_restart_containers INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS total_image_size BIGINT DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS unique_images INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS timezone VARCHAR(100)`,
		`ALTER TABLE image_stats ADD COLUMN IF NOT EXISTS size_bytes BIGINT DEFAULT 0`,
		// Connection and architecture metrics
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS compose_project_count INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_in_compose INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS network_count INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS custom_network_count INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS shared_volume_count INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS containers_with_deps INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS total_dependencies INTEGER DEFAULT 0`,
		`ALTER TABLE telemetry_reports ADD COLUMN IF NOT EXISTS avg_connections_per_container REAL DEFAULT 0.0`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			log.Printf("Migration warning (may be expected): %v", err)
		}
	}

	return nil
}

// Get recent submission events
func (s *Server) handleRecentEvents(w http.ResponseWriter, r *http.Request) {
	limit := getQueryInt(r, "limit", 50)
	since := getQueryInt(r, "since", 0) // ID of last seen event

	var query string
	var args []interface{}

	if since > 0 {
		// Get events newer than the specified ID
		query = `
			SELECT id, installation_id, event_type, timestamp, containers, hosts
			FROM submission_events
			WHERE id > $1
			ORDER BY id DESC
			LIMIT $2
		`
		args = []interface{}{since, limit}
	} else {
		// Get most recent events
		query = `
			SELECT id, installation_id, event_type, timestamp, containers, hosts
			FROM submission_events
			ORDER BY id DESC
			LIMIT $1
		`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	var events []SubmissionEvent
	for rows.Next() {
		var event SubmissionEvent
		if err := rows.Scan(&event.ID, &event.InstallationID, &event.EventType, &event.Timestamp, &event.Containers, &event.Hosts); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		events = append(events, event)
	}

	// Return empty array instead of null if no events
	if events == nil {
		events = []SubmissionEvent{}
	}

	respondJSON(w, http.StatusOK, events)
}

// Get raw database view for debugging and monitoring
func (s *Server) handleDatabaseView(w http.ResponseWriter, r *http.Request) {
	table := r.URL.Query().Get("table")
	limit := getQueryInt(r, "limit", 100)
	offset := getQueryInt(r, "offset", 0)
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")
	installationFilter := r.URL.Query().Get("installation_id")

	// Validate table name to prevent SQL injection
	validTables := map[string]bool{
		"telemetry_reports": true,
		"image_stats":       true,
		"submission_events": true,
	}

	if table == "" {
		table = "telemetry_reports"
	}

	if !validTables[table] {
		respondError(w, http.StatusBadRequest, "Invalid table name")
		return
	}

	// Default sort
	if sortBy == "" {
		if table == "submission_events" {
			sortBy = "id"
		} else {
			sortBy = "timestamp"
		}
	}

	if sortOrder == "" {
		sortOrder = "DESC"
	} else {
		sortOrder = strings.ToUpper(sortOrder)
		if sortOrder != "ASC" && sortOrder != "DESC" {
			sortOrder = "DESC"
		}
	}

	// Build query based on table
	var query string
	var countQuery string
	var args []interface{}
	argNum := 1

	whereClause := ""
	if installationFilter != "" {
		whereClause = fmt.Sprintf(" WHERE installation_id = $%d", argNum)
		args = append(args, installationFilter)
		argNum++
	}

	switch table {
	case "telemetry_reports":
		// Validate sort column for telemetry_reports
		validSortCols := map[string]bool{
			"id": true, "installation_id": true, "timestamp": true,
			"version": true, "total_containers": true, "host_count": true,
		}
		if !validSortCols[sortBy] {
			sortBy = "timestamp"
		}

		query = fmt.Sprintf(`
			SELECT id, installation_id, version, timestamp, host_count, agent_count,
			       total_containers, scan_interval, image_stats, agent_versions,
			       containers_running, containers_stopped, containers_paused, containers_other,
			       avg_cpu_percent, avg_memory_bytes, total_memory_limit,
			       avg_restarts, high_restart_containers, total_image_size, unique_images,
			       timezone, compose_project_count, containers_in_compose,
			       network_count, custom_network_count, shared_volume_count,
			       containers_with_deps, total_dependencies, avg_connections_per_container,
			       created_at
			FROM telemetry_reports
			%s
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortBy, sortOrder, argNum, argNum+1)

		countQuery = fmt.Sprintf("SELECT COUNT(*) FROM telemetry_reports%s", whereClause)

	case "image_stats":
		// Validate sort column for image_stats
		validSortCols := map[string]bool{
			"id": true, "installation_id": true, "timestamp": true,
			"image": true, "count": true, "size_bytes": true,
		}
		if !validSortCols[sortBy] {
			sortBy = "timestamp"
		}

		query = fmt.Sprintf(`
			SELECT id, installation_id, timestamp, image, count, size_bytes, created_at
			FROM image_stats
			%s
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortBy, sortOrder, argNum, argNum+1)

		countQuery = fmt.Sprintf("SELECT COUNT(*) FROM image_stats%s", whereClause)

	case "submission_events":
		// Validate sort column for submission_events
		validSortCols := map[string]bool{
			"id": true, "installation_id": true, "timestamp": true,
			"event_type": true, "containers": true, "hosts": true,
		}
		if !validSortCols[sortBy] {
			sortBy = "id"
		}

		query = fmt.Sprintf(`
			SELECT id, installation_id, event_type, timestamp, containers, hosts, created_at
			FROM submission_events
			%s
			ORDER BY %s %s
			LIMIT $%d OFFSET $%d
		`, whereClause, sortBy, sortOrder, argNum, argNum+1)

		countQuery = fmt.Sprintf("SELECT COUNT(*) FROM submission_events%s", whereClause)
	}

	args = append(args, limit, offset)

	// Execute main query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get columns: "+err.Error())
		return
	}

	// Scan results into generic map structure
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to scan into
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		// Convert to map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle special types
			switch v := val.(type) {
			case []byte:
				// Try to parse as JSON if it's a JSONB column
				if col == "image_stats" || col == "agent_versions" {
					var jsonData interface{}
					if err := json.Unmarshal(v, &jsonData); err == nil {
						rowMap[col] = jsonData
					} else {
						rowMap[col] = string(v)
					}
				} else {
					rowMap[col] = string(v)
				}
			case time.Time:
				rowMap[col] = v.Format(time.RFC3339)
			case nil:
				rowMap[col] = nil
			default:
				rowMap[col] = v
			}
		}

		results = append(results, rowMap)
	}

	// Get total count
	var totalCount int
	countArgs := args[:len(args)-2] // Remove limit and offset
	err = s.db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		log.Printf("Count query error: %v", err)
		totalCount = len(results)
	}

	// Return results with metadata
	response := map[string]interface{}{
		"table":   table,
		"records": results,
		"pagination": map[string]interface{}{
			"total":  totalCount,
			"limit":  limit,
			"offset": offset,
		},
	}

	respondJSON(w, http.StatusOK, response)
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
		log.Printf("⚠️  UPDATE AVAILABLE: Telemetry Collector %s → %s", info.CurrentVersion, info.LatestVersion)
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
