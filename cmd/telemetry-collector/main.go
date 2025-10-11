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
	DatabaseURL string
	Port        int
	APIKey      string // Optional API key for authentication
}

type Server struct {
	db     *sql.DB
	router *mux.Router
	config Config
}

func main() {
	log.Printf("Starting Telemetry Collector Service v%s...", version.Get())

	// Load configuration from environment
	config := Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable"),
		Port:        getEnvInt("PORT", 8081),
		APIKey:      getEnv("API_KEY", ""), // Optional authentication
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
	// Public routes
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/api/ingest", s.handleIngest).Methods("POST")

	// Stats API (read-only, no auth required for public dashboard)
	s.router.HandleFunc("/api/stats/top-images", s.handleTopImages).Methods("GET")
	s.router.HandleFunc("/api/stats/growth", s.handleGrowth).Methods("GET")
	s.router.HandleFunc("/api/stats/installations", s.handleInstallations).Methods("GET")
	s.router.HandleFunc("/api/stats/summary", s.handleSummary).Methods("GET")

	// Serve static files for analytics dashboard
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/analytics")))
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
	// Optional API key authentication
	if s.config.APIKey != "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+s.config.APIKey {
			respondError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}
	}

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
		updateQuery := `
			UPDATE telemetry_reports
			SET version = $2,
			    timestamp = $3,
			    host_count = $4,
			    agent_count = $5,
			    total_containers = $6,
			    scan_interval = $7,
			    image_stats = $8,
			    agent_versions = $9
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
				total_containers, scan_interval, image_stats, agent_versions
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
		)
		if err != nil {
			return fmt.Errorf("failed to insert telemetry: %w", err)
		}

		log.Printf("Inserted new telemetry report for installation %s", report.InstallationID)
	}

	// Insert fresh image stats with normalized names
	for _, imageStat := range report.ImageStats {
		insertImageQuery := `
			INSERT INTO image_stats (installation_id, timestamp, image, count)
			VALUES ($1, $2, $3, $4)
		`
		normalizedImage := normalizeImageName(imageStat.Image)
		_, err := tx.Exec(insertImageQuery, report.InstallationID, report.Timestamp, normalizedImage, imageStat.Count)
		if err != nil {
			log.Printf("Warning: Failed to insert image stat: %v", err)
		}
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
	query := `
		SELECT image, SUM(count) as total_count
		FROM (
			SELECT DISTINCT ON (installation_id, image)
				installation_id,
				image,
				count
			FROM image_stats
			WHERE timestamp >= $1
			ORDER BY installation_id, image, timestamp DESC
		) latest_stats
		GROUP BY image
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

	type Summary struct {
		Installations  int `json:"installations"`
		TotalContainers int `json:"total_containers"`
		TotalHosts     int `json:"total_hosts"`
		TotalAgents    int `json:"total_agents"`
		UniqueImages   int `json:"unique_images"`
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

	respondJSON(w, http.StatusOK, summary)
}

// Helper functions

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
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_image_stats_image ON image_stats(image);
	CREATE INDEX IF NOT EXISTS idx_image_stats_timestamp ON image_stats(timestamp);
	CREATE INDEX IF NOT EXISTS idx_image_stats_installation_id ON image_stats(installation_id);
	`

	_, err := db.Exec(schema)
	return err
}
