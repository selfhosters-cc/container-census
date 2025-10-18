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
	s.router.HandleFunc("/api/stats/growth", s.apiKeyMiddleware(s.handleGrowth)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/installations", s.apiKeyMiddleware(s.handleInstallations)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/summary", s.apiKeyMiddleware(s.handleSummary)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/registries", s.apiKeyMiddleware(s.handleRegistries)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/versions", s.apiKeyMiddleware(s.handleVersions)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/activity-heatmap", s.apiKeyMiddleware(s.handleActivityHeatmap)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/scan-intervals", s.apiKeyMiddleware(s.handleScanIntervals)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/geography", s.apiKeyMiddleware(s.handleGeography)).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/stats/recent-events", s.apiKeyMiddleware(s.handleRecentEvents)).Methods("GET", "OPTIONS")

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

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": version.Get(),
		"time":    time.Now().Format(time.RFC3339),
	})
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
			    timezone = $21
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
				total_image_size, unique_images, timezone
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
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
		)
		if err != nil {
			return fmt.Errorf("failed to insert telemetry: %w", err)
		}

		log.Printf("Inserted new telemetry report for installation %s", report.InstallationID)
	}

	// Insert fresh image stats with normalized names and sizes
	for _, imageStat := range report.ImageStats {
		insertImageQuery := `
			INSERT INTO image_stats (installation_id, timestamp, image, count, size_bytes)
			VALUES ($1, $2, $3, $4, $5)
		`
		normalizedImage := normalizeImageName(imageStat.Image)
		_, err := tx.Exec(insertImageQuery, report.InstallationID, report.Timestamp, normalizedImage, imageStat.Count, imageStat.SizeBytes)
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

	// Deduplicate by using only the most recent image stats per installation
	// This prevents counting the same installation multiple times
	// Apply normalization at query time to handle both old and new data
	query := `
		SELECT normalized_image, SUM(count) as total_count
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
		Image string `json:"image"`
		Count int    `json:"count"`
	}

	var results []ImageCount
	for rows.Next() {
		var ic ImageCount
		if err := rows.Scan(&ic.Image, &ic.Count); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		results = append(results, ic)
	}

	respondJSON(w, http.StatusOK, results)
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
		Installations    int `json:"installations"`
		TotalSubmissions int `json:"total_submissions"`
		TotalContainers  int `json:"total_containers"`
		TotalHosts       int `json:"total_hosts"`
		TotalAgents      int `json:"total_agents"`
		UniqueImages     int `json:"unique_images"`
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
