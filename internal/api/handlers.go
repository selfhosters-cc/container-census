package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/gorilla/mux"
)

// Server handles HTTP requests
type Server struct {
	db         *storage.DB
	scanner    *scanner.Scanner
	router     *mux.Router
	configPath string
}

// New creates a new API server
func New(db *storage.DB, scanner *scanner.Scanner, configPath string) *Server {
	s := &Server{
		db:         db,
		scanner:    scanner,
		router:     mux.NewRouter(),
		configPath: configPath,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// Host endpoints
	api.HandleFunc("/hosts", s.handleGetHosts).Methods("GET")
	api.HandleFunc("/hosts/{id}", s.handleGetHost).Methods("GET")

	// Container endpoints
	api.HandleFunc("/containers", s.handleGetContainers).Methods("GET")
	api.HandleFunc("/containers/host/{id}", s.handleGetContainersByHost).Methods("GET")
	api.HandleFunc("/containers/history", s.handleGetContainersHistory).Methods("GET")
	api.HandleFunc("/containers/{host_id}/{container_id}/start", s.handleStartContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}/stop", s.handleStopContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}/restart", s.handleRestartContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}", s.handleRemoveContainer).Methods("DELETE")
	api.HandleFunc("/containers/{host_id}/{container_id}/logs", s.handleGetLogs).Methods("GET")

	// Image endpoints
	api.HandleFunc("/images", s.handleGetImages).Methods("GET")
	api.HandleFunc("/images/host/{id}", s.handleGetImagesByHost).Methods("GET")
	api.HandleFunc("/images/{host_id}/{image_id}", s.handleRemoveImage).Methods("DELETE")
	api.HandleFunc("/images/host/{id}/prune", s.handlePruneImages).Methods("POST")

	// Scan endpoints
	api.HandleFunc("/scan", s.handleTriggerScan).Methods("POST")
	api.HandleFunc("/scan/results", s.handleGetScanResults).Methods("GET")
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Config endpoints
	api.HandleFunc("/config/reload", s.handleReloadConfig).Methods("POST")

	// Serve static files (embedded web frontend)
	s.router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))
}

// Router returns the configured router
func (s *Server) Router() *mux.Router {
	return s.router
}

// API Handlers

func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := s.db.GetHost(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	respondJSON(w, http.StatusOK, host)
}

func (s *Server) handleGetContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := s.db.GetLatestContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, containers)
}

func (s *Server) handleGetContainersByHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containers, err := s.db.GetContainersByHost(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, containers)
}

func (s *Server) handleGetContainersHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for time range
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid start time format")
			return
		}
	} else {
		// Default to last 24 hours
		start = time.Now().Add(-24 * time.Hour)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid end time format")
			return
		}
	} else {
		end = time.Now()
	}

	containers, err := s.db.GetContainersHistory(start, end)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers history: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, containers)
}

func (s *Server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	// Get all hosts
	hosts, err := s.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	// Trigger scan in background
	go func() {
		ctx := context.Background()
		for _, host := range hosts {
			if !host.Enabled {
				continue
			}

			result := models.ScanResult{
				HostID:    host.ID,
				HostName:  host.Name,
				StartedAt: time.Now(),
			}

			containers, err := s.scanner.ScanHost(ctx, host)
			result.CompletedAt = time.Now()

			if err != nil {
				result.Success = false
				result.Error = err.Error()
				log.Printf("Scan failed for host %s: %v", host.Name, err)
			} else {
				result.Success = true
				result.ContainersFound = len(containers)

				// Save containers
				if err := s.db.SaveContainers(containers); err != nil {
					log.Printf("Failed to save containers for host %s: %v", host.Name, err)
				}
			}

			// Save scan result
			if _, err := s.db.SaveScanResult(result); err != nil {
				log.Printf("Failed to save scan result for host %s: %v", host.Name, err)
			}
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{"message": "Scan triggered"})
}

func (s *Server) handleGetScanResults(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	results, err := s.db.GetScanResults(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get scan results: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, results)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Helper functions

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

// Container Management Handlers

func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containerID := vars["container_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	ctx := r.Context()
	if err := s.scanner.StartContainer(ctx, *host, containerID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container started"})
}

func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containerID := vars["container_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Get timeout from query param (default 10 seconds)
	timeout := 10
	if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	ctx := r.Context()
	if err := s.scanner.StopContainer(ctx, *host, containerID, timeout); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to stop container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container stopped"})
}

func (s *Server) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containerID := vars["container_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Get timeout from query param (default 10 seconds)
	timeout := 10
	if timeoutStr := r.URL.Query().Get("timeout"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	ctx := r.Context()
	if err := s.scanner.RestartContainer(ctx, *host, containerID, timeout); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to restart container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container restarted"})
}

func (s *Server) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containerID := vars["container_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Get force from query param (default false)
	force := r.URL.Query().Get("force") == "true"

	ctx := r.Context()
	if err := s.scanner.RemoveContainer(ctx, *host, containerID, force); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove container: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Container removed"})
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	containerID := vars["container_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Get tail from query param (default 100 lines)
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	ctx := r.Context()
	logs, err := s.scanner.GetContainerLogs(ctx, *host, containerID, tail)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get logs: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"logs": logs})
}

// Image Management Handlers

func (s *Server) handleGetImages(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	ctx := r.Context()
	allImages := make(map[string]interface{})

	for _, host := range hosts {
		if !host.Enabled {
			continue
		}

		images, err := s.scanner.ListImages(ctx, host)
		if err != nil {
			log.Printf("Failed to list images for host %s: %v", host.Name, err)
			continue
		}

		allImages[host.Name] = map[string]interface{}{
			"host_id": host.ID,
			"images":  images,
		}
	}

	respondJSON(w, http.StatusOK, allImages)
}

func (s *Server) handleGetImagesByHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	ctx := r.Context()
	images, err := s.scanner.ListImages(ctx, *host)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list images: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, images)
}

func (s *Server) handleRemoveImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["host_id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	imageID := vars["image_id"]

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	// Get force from query param (default false)
	force := r.URL.Query().Get("force") == "true"

	ctx := r.Context()
	if err := s.scanner.RemoveImage(ctx, *host, imageID, force); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove image: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Image removed"})
}

func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := s.db.GetHost(hostID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Host not found")
		return
	}

	ctx := r.Context()
	spaceReclaimed, err := s.scanner.PruneImages(ctx, *host)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to prune images: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":         "Images pruned",
		"space_reclaimed": spaceReclaimed,
	})
}

// handleReloadConfig reloads configuration and syncs hosts
func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	log.Println("Reloading configuration...")

	// Load config
	cfg := config.LoadOrDefault(s.configPath)

	// Sync hosts from config
	added := 0
	updated := 0
	errors := []string{}

	for _, hc := range cfg.Hosts {
		// Check if host exists by name
		hosts, err := s.db.GetHosts()
		if err != nil {
			log.Printf("Failed to get hosts: %v", err)
			errors = append(errors, err.Error())
			continue
		}

		found := false
		for _, existing := range hosts {
			if existing.Name == hc.Name {
				// Host exists, update if changed
				if existing.Address != hc.Address || existing.Description != hc.Description {
					existing.Address = hc.Address
					existing.Description = hc.Description
					if err := s.db.UpdateHost(existing); err != nil {
						log.Printf("Failed to update host %s: %v", hc.Name, err)
						errors = append(errors, "Failed to update "+hc.Name+": "+err.Error())
					} else {
						log.Printf("Updated host: %s", hc.Name)
						updated++
					}
				}
				found = true
				break
			}
		}

		if !found {
			// Add new host
			host := models.Host{
				Name:        hc.Name,
				Address:     hc.Address,
				Description: hc.Description,
				Enabled:     true,
			}

			id, err := s.db.AddHost(host)
			if err != nil {
				log.Printf("Failed to add host %s: %v", hc.Name, err)
				errors = append(errors, "Failed to add "+hc.Name+": "+err.Error())
			} else {
				log.Printf("Added host: %s (ID: %d)", hc.Name, id)
				added++
			}
		}
	}

	result := map[string]interface{}{
		"message": "Configuration reloaded",
		"added":   added,
		"updated": updated,
	}

	if len(errors) > 0 {
		result["errors"] = errors
	}

	respondJSON(w, http.StatusOK, result)
}
