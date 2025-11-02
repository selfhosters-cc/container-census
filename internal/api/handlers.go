package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/auth"
	"github.com/container-census/container-census/internal/config"
	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/notifications"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
	"github.com/container-census/container-census/internal/telemetry"
	"github.com/container-census/container-census/internal/version"
	"github.com/gorilla/mux"
)

// Server handles HTTP requests
type Server struct {
	db                    *storage.DB
	scanner               *scanner.Scanner
	router                *mux.Router
	configPath            string
	telemetryScheduler    *telemetry.Scheduler
	telemetryContext      context.Context
	telemetryCancel       context.CancelFunc
	telemetryMutex        sync.Mutex
	scanInterval          int
	authConfig            auth.Config
	setScanIntervalFunc   func(int) // Callback to update scan interval
	notificationService   *notifications.NotificationService
	vulnScanner           VulnerabilityScanner
	vulnScheduler         VulnerabilityScheduler
}

// TelemetryScheduler interface for submitting telemetry on demand
type TelemetryScheduler interface {
	SubmitNow(ctx context.Context) error
	Stop()
}

// New creates a new API server
func New(db *storage.DB, scanner *scanner.Scanner, configPath string, scanInterval int, authConfig auth.Config) *Server {
	s := &Server{
		db:           db,
		scanner:      scanner,
		router:       mux.NewRouter(),
		configPath:   configPath,
		scanInterval: scanInterval,
		authConfig:   authConfig,
	}

	s.setupRoutes()
	return s
}

// SetScanIntervalCallback sets the callback function to update scan interval dynamically
func (s *Server) SetScanIntervalCallback(callback func(int)) {
	s.setScanIntervalFunc = callback
}

// SetTelemetryScheduler sets the telemetry scheduler for on-demand submissions
func (s *Server) SetTelemetryScheduler(scheduler *telemetry.Scheduler, ctx context.Context, cancel context.CancelFunc) {
	s.telemetryMutex.Lock()
	defer s.telemetryMutex.Unlock()

	s.telemetryScheduler = scheduler
	s.telemetryContext = ctx
	s.telemetryCancel = cancel
}

// SetNotificationService sets the notification service
func (s *Server) SetNotificationService(ns *notifications.NotificationService) {
	s.notificationService = ns
}

// RestartTelemetry stops and restarts the telemetry scheduler with new configuration
func (s *Server) RestartTelemetry() error {
	s.telemetryMutex.Lock()
	defer s.telemetryMutex.Unlock()

	// Stop existing scheduler if running
	if s.telemetryScheduler != nil && s.telemetryCancel != nil {
		log.Println("Stopping existing telemetry scheduler...")
		s.telemetryCancel()
		s.telemetryScheduler = nil
		s.telemetryContext = nil
		s.telemetryCancel = nil
	}

	// Load new config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Count enabled endpoints
	enabledCount := 0
	for _, ep := range cfg.Telemetry.Endpoints {
		if ep.Enabled {
			enabledCount++
		}
	}

	// Only start if there are enabled endpoints
	if enabledCount == 0 {
		log.Println("No enabled telemetry endpoints, not starting scheduler")
		return nil
	}

	// Create new scheduler
	newScheduler, err := telemetry.NewScheduler(s.db, s.scanner, cfg.Telemetry, s.scanInterval)
	if err != nil {
		return err
	}

	// Create new context
	ctx, cancel := context.WithCancel(context.Background())

	// Set the new scheduler
	s.telemetryScheduler = newScheduler
	s.telemetryContext = ctx
	s.telemetryCancel = cancel

	// Start it in a goroutine
	go newScheduler.Start(ctx)

	log.Printf("Telemetry scheduler restarted with %d enabled endpoint(s) - interval: %dh", enabledCount, cfg.Telemetry.IntervalHours)
	return nil
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Apply authentication middleware to all routes
	authMiddleware := auth.BasicAuthMiddleware(s.authConfig)

	// Health endpoint without authentication (for monitoring)
	s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET")

	// Protected API routes
	api := s.router.PathPrefix("/api").Subrouter()
	api.Use(authMiddleware)

	// Host endpoints
	api.HandleFunc("/hosts", s.handleGetHosts).Methods("GET")
	api.HandleFunc("/hosts/{id}", s.handleGetHost).Methods("GET")
	api.HandleFunc("/hosts/{id}", s.handleUpdateHost).Methods("PUT")
	api.HandleFunc("/hosts/{id}", s.handleDeleteHost).Methods("DELETE")
	api.HandleFunc("/hosts/agent", s.handleAddAgentHost).Methods("POST")
	api.HandleFunc("/hosts/agent/test", s.handleTestAgentConnection).Methods("POST")
	api.HandleFunc("/hosts/agent/{id}/info", s.handleGetAgentInfo).Methods("GET")

	// Container endpoints
	api.HandleFunc("/containers", s.handleGetContainers).Methods("GET")
	api.HandleFunc("/containers/graph", s.handleGetContainerGraph).Methods("GET")
	api.HandleFunc("/containers/host/{id}", s.handleGetContainersByHost).Methods("GET")
	api.HandleFunc("/containers/history", s.handleGetContainersHistory).Methods("GET")
	api.HandleFunc("/containers/lifecycle", s.handleGetContainerLifecycles).Methods("GET")
	api.HandleFunc("/containers/lifecycle/{host_id}/{container_name}", s.handleGetContainerLifecycleEvents).Methods("GET")
	api.HandleFunc("/containers/{host_id}/{container_id}/stats", s.handleGetContainerStats).Methods("GET")
	api.HandleFunc("/containers/{host_id}/{container_id}/start", s.handleStartContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}/stop", s.handleStopContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}/restart", s.handleRestartContainer).Methods("POST")
	api.HandleFunc("/containers/{host_id}/{container_id}", s.handleRemoveContainer).Methods("DELETE")
	api.HandleFunc("/containers/{host_id}/{container_id}/logs", s.handleGetLogs).Methods("GET")

	// Prometheus metrics endpoint (protected)
	api.HandleFunc("/metrics", s.handlePrometheusMetrics).Methods("GET")

	// Image endpoints
	api.HandleFunc("/images", s.handleGetImages).Methods("GET")
	api.HandleFunc("/images/host/{id}", s.handleGetImagesByHost).Methods("GET")
	api.HandleFunc("/images/{host_id}/{image_id}", s.handleRemoveImage).Methods("DELETE")
	api.HandleFunc("/images/host/{id}/prune", s.handlePruneImages).Methods("POST")

	// Scan endpoints
	api.HandleFunc("/scan", s.handleTriggerScan).Methods("POST")
	api.HandleFunc("/scan/results", s.handleGetScanResults).Methods("GET")

	// Activity log (scans + telemetry)
	api.HandleFunc("/activity-log", s.handleGetActivityLog).Methods("GET")

	// Reports endpoints
	api.HandleFunc("/reports/changes", s.handleGetChangesReport).Methods("GET")

	// Config endpoints
	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config/scanner", s.handleUpdateScanner).Methods("POST")
	api.HandleFunc("/config/telemetry", s.handleUpdateTelemetry).Methods("POST")
	api.HandleFunc("/config/telemetry/endpoints", s.handleGetTelemetryEndpoints).Methods("GET")
	api.HandleFunc("/config/telemetry/endpoints", s.handleAddTelemetryEndpoint).Methods("POST")
	api.HandleFunc("/config/telemetry/endpoints/{name}", s.handleUpdateTelemetryEndpoint).Methods("PUT")
	api.HandleFunc("/config/telemetry/endpoints/{name}", s.handleDeleteTelemetryEndpoint).Methods("DELETE")

	// Telemetry endpoints
	api.HandleFunc("/telemetry/submit", s.handleSubmitTelemetry).Methods("POST")
	api.HandleFunc("/telemetry/status", s.handleGetTelemetryStatus).Methods("GET")
	api.HandleFunc("/telemetry/schedule", s.handleGetTelemetrySchedule).Methods("GET")
	api.HandleFunc("/telemetry/reset-circuit-breaker/{name}", s.handleResetCircuitBreaker).Methods("POST")
	api.HandleFunc("/telemetry/debug-enabled", s.handleGetDebugEnabled).Methods("GET")
	api.HandleFunc("/telemetry/test-endpoint", s.handleTestTelemetryEndpoint).Methods("POST")

	// Notification endpoints
	api.HandleFunc("/notifications/channels", s.handleGetNotificationChannels).Methods("GET")
	api.HandleFunc("/notifications/channels", s.handleCreateNotificationChannel).Methods("POST")
	api.HandleFunc("/notifications/channels/{id}", s.handleUpdateNotificationChannel).Methods("PUT")
	api.HandleFunc("/notifications/channels/{id}", s.handleDeleteNotificationChannel).Methods("DELETE")
	api.HandleFunc("/notifications/channels/{id}/test", s.handleTestNotificationChannel).Methods("POST")

	api.HandleFunc("/notifications/rules", s.handleGetNotificationRules).Methods("GET")
	api.HandleFunc("/notifications/rules", s.handleCreateNotificationRule).Methods("POST")
	api.HandleFunc("/notifications/rules/{id}", s.handleUpdateNotificationRule).Methods("PUT")
	api.HandleFunc("/notifications/rules/{id}", s.handleDeleteNotificationRule).Methods("DELETE")

	api.HandleFunc("/notifications/logs", s.handleGetNotificationLogs).Methods("GET")
	api.HandleFunc("/notifications/logs/{id}/read", s.handleMarkNotificationRead).Methods("PUT")
	api.HandleFunc("/notifications/logs/read-all", s.handleMarkAllNotificationsRead).Methods("PUT")
	api.HandleFunc("/notifications/logs/clear", s.handleClearNotifications).Methods("DELETE")

	api.HandleFunc("/notifications/silences", s.handleGetNotificationSilences).Methods("GET")
	api.HandleFunc("/notifications/silences", s.handleCreateNotificationSilence).Methods("POST")
	api.HandleFunc("/notifications/silences/{id}", s.handleDeleteNotificationSilence).Methods("DELETE")

	api.HandleFunc("/notifications/status", s.handleGetNotificationStatus).Methods("GET")

	// Vulnerability endpoints
	api.HandleFunc("/vulnerabilities/summary", s.handleGetVulnerabilitySummary).Methods("GET")
	api.HandleFunc("/vulnerabilities/scans", s.handleGetAllVulnerabilityScans).Methods("GET")
	api.HandleFunc("/vulnerabilities/image/{imageId}", s.handleGetImageVulnerabilities).Methods("GET")
	api.HandleFunc("/vulnerabilities/container/{hostId}/{containerId}", s.handleGetContainerVulnerabilities).Methods("GET")
	api.HandleFunc("/vulnerabilities/scan/{imageId}", s.handleTriggerImageScan).Methods("POST")
	api.HandleFunc("/vulnerabilities/scan-all", s.handleTriggerScanAll).Methods("POST")
	api.HandleFunc("/vulnerabilities/queue", s.handleGetScanQueue).Methods("GET")
	api.HandleFunc("/vulnerabilities/update-db", s.handleUpdateTrivyDB).Methods("POST")
	api.HandleFunc("/vulnerabilities/settings", s.handleGetVulnerabilitySettings).Methods("GET")
	api.HandleFunc("/vulnerabilities/settings", s.handleUpdateVulnerabilitySettings).Methods("PUT")

	// Serve static files (embedded web frontend) - also protected
	s.router.PathPrefix("/").Handler(authMiddleware(http.FileServer(http.Dir("./web"))))
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

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	var host models.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	host.ID = id
	if err := s.db.UpdateHost(host); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update host: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Host updated successfully"})
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	if err := s.db.DeleteHost(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete host: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Host deleted successfully"})
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

func (s *Server) handleGetContainerLifecycles(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	hostFilterStr := r.URL.Query().Get("host_id")

	limit := 100 // default
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	hostFilter := int64(0) // 0 means all hosts
	if hostFilterStr != "" {
		parsedHost, err := strconv.ParseInt(hostFilterStr, 10, 64)
		if err == nil {
			hostFilter = parsedHost
		}
	}

	summaries, err := s.db.GetContainerLifecycleSummaries(limit, hostFilter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get container lifecycles: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleGetContainerLifecycleEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerName := vars["container_name"]
	hostIDStr := vars["host_id"]

	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	events, err := s.db.GetContainerLifecycleEvents(containerName, hostID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get container lifecycle events: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, events)
}

func (s *Server) handleGetContainerGraph(w http.ResponseWriter, r *http.Request) {
	// Get latest containers with all connection details
	containers, err := s.db.GetLatestContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers: "+err.Error())
		return
	}

	// Build graph nodes and edges
	graph := models.ContainerGraph{
		Nodes: make([]models.ContainerGraphNode, 0, len(containers)),
		Edges: make([]models.ContainerGraphEdge, 0),
	}

	// Create container nodes
	for _, c := range containers {
		node := models.ContainerGraphNode{
			ID:             c.ID,
			Name:           c.Name,
			NodeType:       "container",
			Image:          c.Image,
			State:          c.State,
			HostID:         c.HostID,
			HostName:       c.HostName,
			ComposeProject: c.ComposeProject,
		}
		graph.Nodes = append(graph.Nodes, node)
	}

	// Create network nodes by collecting all unique networks
	networkMap := make(map[string]map[int64]bool) // network name -> set of host IDs
	for _, c := range containers {
		for _, network := range c.Networks {
			if networkMap[network] == nil {
				networkMap[network] = make(map[int64]bool)
			}
			networkMap[network][c.HostID] = true
		}
	}

	// Create a network node for each unique network+host combination
	networkNodeIDs := make(map[string]string) // network+host -> node ID
	for networkName, hostIDs := range networkMap {
		for hostID := range hostIDs {
			// Create a unique ID for this network on this host
			networkNodeID := fmt.Sprintf("net-%d-%s", hostID, networkName)
			networkNodeIDs[fmt.Sprintf("%d-%s", hostID, networkName)] = networkNodeID

			// Find host name for this network node
			var hostName string
			for _, c := range containers {
				if c.HostID == hostID {
					hostName = c.HostName
					break
				}
			}

			graph.Nodes = append(graph.Nodes, models.ContainerGraphNode{
				ID:       networkNodeID,
				Name:     networkName,
				NodeType: "network",
				HostID:   hostID,
				HostName: hostName,
			})
		}
	}

	// Build edges by analyzing connections
	// Track which connections we've already added to avoid duplicates
	edgeMap := make(map[string]bool)

	for i, c1 := range containers {
		// Network connections - connect each container to its network nodes
		for _, network := range c1.Networks {
			// Get the network node ID for this network on this host
			networkKey := fmt.Sprintf("%d-%s", c1.HostID, network)
			if networkNodeID, exists := networkNodeIDs[networkKey]; exists {
				edgeKey := c1.ID + "-" + networkNodeID + "-network"
				if !edgeMap[edgeKey] {
					graph.Edges = append(graph.Edges, models.ContainerGraphEdge{
						Source: c1.ID,
						Target: networkNodeID,
						Type:   "network",
						Label:  "", // No label needed since network node itself has the name
					})
					edgeMap[edgeKey] = true
				}
			}
		}

		// Volume connections (shared volumes)
		for _, vol1 := range c1.Volumes {
			if vol1.Type != "volume" || vol1.Name == "" {
				continue // Only process named volumes
			}
			// Find other containers with the same volume on the same host
			for j, c2 := range containers {
				if i >= j {
					continue
				}
				// Volumes are isolated per Docker daemon - only connect containers on same host
				if c1.HostID != c2.HostID {
					continue
				}
				for _, vol2 := range c2.Volumes {
					if vol1.Name == vol2.Name && vol1.Type == vol2.Type {
						edgeKey := c1.ID + "-" + c2.ID + "-volume-" + vol1.Name
						if !edgeMap[edgeKey] {
							graph.Edges = append(graph.Edges, models.ContainerGraphEdge{
								Source: c1.ID,
								Target: c2.ID,
								Type:   "volume",
								Label:  vol1.Name,
							})
							edgeMap[edgeKey] = true
						}
					}
				}
			}
		}

		// Legacy links
		for _, link := range c1.Links {
			// Links are in format: /container_name:/alias
			// Extract the target container name
			parts := strings.Split(link, ":")
			if len(parts) > 0 {
				targetName := strings.TrimPrefix(parts[0], "/")
				// Find the target container by name on the same host
				for _, c2 := range containers {
					// Links only work on same host
					if c1.HostID != c2.HostID {
						continue
					}
					if c2.Name == targetName {
						edgeKey := c1.ID + "-" + c2.ID + "-link"
						if !edgeMap[edgeKey] {
							graph.Edges = append(graph.Edges, models.ContainerGraphEdge{
								Source: c1.ID,
								Target: c2.ID,
								Type:   "link",
								Label:  "linked",
							})
							edgeMap[edgeKey] = true
						}
						break
					}
				}
			}
		}

		// Docker Compose depends_on from labels
		if dependsOn, ok := c1.Labels["com.docker.compose.depends_on"]; ok && dependsOn != "" {
			// Format: "service1:condition:required,service2:condition:required"
			dependencies := strings.Split(dependsOn, ",")
			for _, dep := range dependencies {
				// Parse "service:condition:required"
				depParts := strings.Split(strings.TrimSpace(dep), ":")
				if len(depParts) > 0 {
					targetService := depParts[0]
					// Find container with matching compose service name on same host
					for _, c2 := range containers {
						if serviceName, ok := c2.Labels["com.docker.compose.service"]; ok && serviceName == targetService {
							// Only create edge if same compose project AND same host
							if c1.ComposeProject != "" && c1.ComposeProject == c2.ComposeProject && c1.HostID == c2.HostID {
								edgeKey := c1.ID + "-" + c2.ID + "-depends"
								if !edgeMap[edgeKey] {
									graph.Edges = append(graph.Edges, models.ContainerGraphEdge{
										Source: c1.ID,
										Target: c2.ID,
										Type:   "depends",
										Label:  "depends on",
									})
									edgeMap[edgeKey] = true
								}
							}
							break
						}
					}
				}
			}
		}
	}

	respondJSON(w, http.StatusOK, graph)
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

func (s *Server) handleGetActivityLog(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	activityType := r.URL.Query().Get("type")
	if activityType == "" {
		activityType = "all" // default to all activities
	}

	// Validate activity type
	if activityType != "all" && activityType != "scan" && activityType != "telemetry" {
		respondError(w, http.StatusBadRequest, "Invalid type parameter. Must be 'all', 'scan', or 'telemetry'")
		return
	}

	activities, err := s.db.GetActivityLog(limit, activityType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get activity log: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, activities)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
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
	cfg, _ := config.LoadOrDefault(s.configPath)

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

// handleSubmitTelemetry triggers an immediate telemetry submission
func (s *Server) handleSubmitTelemetry(w http.ResponseWriter, r *http.Request) {
	s.telemetryMutex.Lock()
	scheduler := s.telemetryScheduler
	s.telemetryMutex.Unlock()

	if scheduler == nil {
		// Check if there are any enabled endpoints configured
		cfg, _ := config.LoadOrDefault(s.configPath)

		// Count enabled endpoints
		enabledCount := 0
		for _, ep := range cfg.Telemetry.Endpoints {
			if ep.Enabled {
				enabledCount++
			}
		}

		if enabledCount == 0 {
			respondError(w, http.StatusServiceUnavailable, "No telemetry endpoints configured or enabled. Please enable community telemetry or add a private collector in Settings.")
			return
		}

		// Scheduler not running - try to start it
		log.Printf("Telemetry scheduler not running, attempting to start with %d enabled endpoint(s)...", enabledCount)
		if err := s.RestartTelemetry(); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to initialize telemetry: "+err.Error())
			return
		}

		// Get the scheduler again after restart
		s.telemetryMutex.Lock()
		scheduler = s.telemetryScheduler
		s.telemetryMutex.Unlock()

		if scheduler == nil {
			respondError(w, http.StatusServiceUnavailable, "Telemetry scheduler failed to initialize")
			return
		}
	}

	// Trigger submission asynchronously so we don't block the HTTP response
	// Use a background context with timeout instead of the request context
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := scheduler.SubmitNow(ctx); err != nil {
			log.Printf("Manual telemetry submission failed: %v", err)
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Telemetry submission triggered successfully",
	})
}

// handleGetConfig returns the current configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.LoadOrDefault(s.configPath)
	respondJSON(w, http.StatusOK, cfg)
}

// handleUpdateScanner updates scanner settings
func (s *Server) handleUpdateScanner(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IntervalSeconds int `json:"interval_seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.IntervalSeconds < 30 {
		respondError(w, http.StatusBadRequest, "Interval must be at least 30 seconds")
		return
	}

	// Load current config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Update scan interval
	cfg.Scanner.IntervalSeconds = req.IntervalSeconds

	// Save configuration
	if err := config.Save(s.configPath, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	log.Printf("Scanner interval updated to %d seconds", req.IntervalSeconds)

	// Update the scanner's interval in memory
	s.scanInterval = req.IntervalSeconds

	// If callback is set, notify the background scanner to update its ticker
	if s.setScanIntervalFunc != nil {
		s.setScanIntervalFunc(req.IntervalSeconds)
		log.Printf("Notified background scanner of interval change")
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Scanner interval updated successfully. Will take effect on next scan cycle.",
	})
}

// handleUpdateTelemetry updates telemetry settings
func (s *Server) handleUpdateTelemetry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled           bool `json:"enabled"`
		IntervalHours     int  `json:"interval_hours"`
		CommunityEndpoint bool `json:"community_endpoint"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Load current config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Update interval
	if req.IntervalHours > 0 {
		cfg.Telemetry.IntervalHours = req.IntervalHours
	}

	// Update or add community endpoint
	communityURL := "https://cc-telemetry.selfhosters.cc/api/ingest"
	foundCommunity := false

	for i := range cfg.Telemetry.Endpoints {
		if cfg.Telemetry.Endpoints[i].URL == communityURL {
			cfg.Telemetry.Endpoints[i].Enabled = req.CommunityEndpoint
			foundCommunity = true
			break
		}
	}

	if !foundCommunity && req.CommunityEndpoint {
		// Add community endpoint
		cfg.Telemetry.Endpoints = append(cfg.Telemetry.Endpoints, models.TelemetryEndpoint{
			Name:    "community",
			URL:     communityURL,
			Enabled: true,
		})
	}

	// Save configuration (write to config file)
	if err := config.Save(s.configPath, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	// Log the update for server logs
	log.Printf("Telemetry settings updated - interval: %dh, community: %v",
		cfg.Telemetry.IntervalHours, req.CommunityEndpoint)

	// Restart telemetry scheduler to apply changes immediately
	if err := s.RestartTelemetry(); err != nil {
		log.Printf("Warning: Failed to restart telemetry: %v", err)
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Settings saved but failed to restart telemetry: " + err.Error(),
			"warning": "true",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Telemetry settings updated and applied successfully",
	})
}

// Telemetry Endpoint Management Handlers

// handleGetTelemetryEndpoints returns all configured telemetry endpoints
func (s *Server) handleGetTelemetryEndpoints(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Return empty array instead of null if no endpoints
	endpoints := cfg.Telemetry.Endpoints
	if endpoints == nil {
		endpoints = []models.TelemetryEndpoint{}
	}

	respondJSON(w, http.StatusOK, endpoints)
}

// handleAddTelemetryEndpoint adds a new custom telemetry endpoint
func (s *Server) handleAddTelemetryEndpoint(w http.ResponseWriter, r *http.Request) {
	var endpoint models.TelemetryEndpoint
	if err := json.NewDecoder(r.Body).Decode(&endpoint); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate required fields
	if endpoint.Name == "" {
		respondError(w, http.StatusBadRequest, "Endpoint name is required")
		return
	}
	if endpoint.URL == "" {
		respondError(w, http.StatusBadRequest, "Endpoint URL is required")
		return
	}

	// Load current config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Check if endpoint with same name already exists
	for _, ep := range cfg.Telemetry.Endpoints {
		if ep.Name == endpoint.Name {
			respondError(w, http.StatusConflict, "Endpoint with this name already exists")
			return
		}
	}

	// Add the new endpoint
	cfg.Telemetry.Endpoints = append(cfg.Telemetry.Endpoints, endpoint)

	// Save configuration
	if err := config.Save(s.configPath, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	log.Printf("Telemetry endpoint added: %s (%s) enabled: %v", endpoint.Name, endpoint.URL, endpoint.Enabled)

	// Restart telemetry scheduler to apply changes immediately
	if err := s.RestartTelemetry(); err != nil {
		log.Printf("Warning: Failed to restart telemetry: %v", err)
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Telemetry endpoint added successfully",
	})
}

// handleUpdateTelemetryEndpoint updates an existing telemetry endpoint
func (s *Server) handleUpdateTelemetryEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var updatedEndpoint models.TelemetryEndpoint
	if err := json.NewDecoder(r.Body).Decode(&updatedEndpoint); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Load current config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Find and update the endpoint
	found := false
	for i := range cfg.Telemetry.Endpoints {
		if cfg.Telemetry.Endpoints[i].Name == name {
			// ONLY update the enabled field - don't touch anything else
			cfg.Telemetry.Endpoints[i].Enabled = updatedEndpoint.Enabled
			found = true
			break
		}
	}

	if !found {
		respondError(w, http.StatusNotFound, "Telemetry endpoint not found")
		return
	}

	// Save configuration
	if err := config.Save(s.configPath, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	log.Printf("Telemetry endpoint updated: %s (enabled: %v)", name, updatedEndpoint.Enabled)

	// Restart telemetry scheduler to apply changes immediately
	if err := s.RestartTelemetry(); err != nil {
		log.Printf("Warning: Failed to restart telemetry: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Telemetry endpoint updated successfully",
	})
}

// handleDeleteTelemetryEndpoint deletes a telemetry endpoint
func (s *Server) handleDeleteTelemetryEndpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Load current config
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Find and remove the endpoint
	found := false
	newEndpoints := []models.TelemetryEndpoint{}
	for _, ep := range cfg.Telemetry.Endpoints {
		if ep.Name == name {
			found = true
			continue // Skip this endpoint (delete it)
		}
		newEndpoints = append(newEndpoints, ep)
	}

	if !found {
		respondError(w, http.StatusNotFound, "Telemetry endpoint not found")
		return
	}

	cfg.Telemetry.Endpoints = newEndpoints

	// Save configuration
	if err := config.Save(s.configPath, cfg); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	log.Printf("Telemetry endpoint deleted: %s", name)

	// Restart telemetry scheduler to apply changes immediately
	if err := s.RestartTelemetry(); err != nil {
		log.Printf("Warning: Failed to restart telemetry: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Telemetry endpoint deleted successfully",
	})
}

// handleGetTelemetryStatus returns the telemetry submission status for all endpoints
func (s *Server) handleGetTelemetryStatus(w http.ResponseWriter, r *http.Request) {
	// Load current config to get all endpoints
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Get telemetry statuses from database
	statuses, err := s.db.GetAllTelemetryStatuses()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get telemetry status: "+err.Error())
		return
	}

	// Merge config endpoints with status data
	result := make([]models.TelemetryEndpoint, 0, len(cfg.Telemetry.Endpoints))
	for _, endpoint := range cfg.Telemetry.Endpoints {
		// Copy endpoint from config
		ep := endpoint

		// Merge with status from database if available
		if status, exists := statuses[endpoint.Name]; exists {
			ep.LastSuccess = status.LastSuccess
			ep.LastFailure = status.LastFailure
			ep.LastFailureReason = status.LastFailureReason
		}

		result = append(result, ep)
	}

	respondJSON(w, http.StatusOK, result)
}

// handleResetCircuitBreaker clears the failure status for a telemetry endpoint (resets circuit breaker)
func (s *Server) handleResetCircuitBreaker(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Load current config to verify the endpoint exists
	cfg, _ := config.LoadOrDefault(s.configPath)

	// Check if endpoint exists in config
	found := false
	for _, ep := range cfg.Telemetry.Endpoints {
		if ep.Name == name {
			found = true
			break
		}
	}

	if !found {
		respondError(w, http.StatusNotFound, "Telemetry endpoint not found")
		return
	}

	// Clear the failure status
	if err := s.db.ClearTelemetryFailure(name); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to reset circuit breaker: "+err.Error())
		return
	}

	log.Printf("Circuit breaker reset for telemetry endpoint: %s", name)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Circuit breaker reset successfully",
		"endpoint": name,
	})
}

// handleGetDebugEnabled checks if debug features are enabled via environment variable
func (s *Server) handleGetDebugEnabled(w http.ResponseWriter, r *http.Request) {
	// Check for ENABLE_TELEMETRY_DEBUG or TELEMETRY_DEBUG environment variable
	debugEnabled := os.Getenv("ENABLE_TELEMETRY_DEBUG") == "true" || os.Getenv("TELEMETRY_DEBUG") == "true"

	respondJSON(w, http.StatusOK, map[string]bool{
		"debug_enabled": debugEnabled,
	})
}

// handleTestTelemetryEndpoint tests a telemetry collector endpoint connection
func (s *Server) handleTestTelemetryEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required")
		return
	}

	// Create a test telemetry report with minimal data
	testReport := models.TelemetryReport{
		InstallationID:  "test-connection",
		Version:         version.Get(),
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 0,
		ImageStats:      []models.ImageStat{},
		Timezone:        "UTC",
	}

	// Try to submit to the endpoint
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}
	jsonData, err := json.Marshal(testReport)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create test payload")
		return
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.URL, strings.NewReader(string(jsonData)))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid URL: "+err.Error())
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("X-API-Key", req.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		respondError(w, http.StatusBadGateway, "Connection failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("Collector returned status %d", resp.StatusCode))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Connection successful",
		"status":  fmt.Sprintf("%d", resp.StatusCode),
	})
}

// handleGetTelemetrySchedule returns information about the next scheduled telemetry submission
func (s *Server) handleGetTelemetrySchedule(w http.ResponseWriter, r *http.Request) {
	if s.telemetryScheduler == nil {
		// No scheduler running - return basic info from config
		cfg, _ := config.LoadOrDefault(s.configPath)
		enabledCount := 0
		for _, ep := range cfg.Telemetry.Endpoints {
			if ep.Enabled {
				enabledCount++
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled_endpoints": enabledCount,
			"interval_hours":    cfg.Telemetry.IntervalHours,
			"next_submission":   nil,
			"message":           "Telemetry scheduler not running",
		})
		return
	}

	scheduleInfo := s.telemetryScheduler.GetScheduleInfo()
	respondJSON(w, http.StatusOK, scheduleInfo)
}

// handleGetContainerStats returns time-series stats for a specific container
func (s *Server) handleGetContainerStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostIDStr := vars["host_id"]
	containerID := vars["container_id"]

	hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	// Parse time range parameter
	rangeParam := r.URL.Query().Get("range")
	var hoursBack int

	switch rangeParam {
	case "1h":
		hoursBack = 1
	case "24h":
		hoursBack = 24
	case "7d":
		hoursBack = 24 * 7 // 168 hours
	case "all", "":
		hoursBack = 0 // 0 means all data
	default:
		respondError(w, http.StatusBadRequest, "Invalid range parameter. Use: 1h, 24h, 7d, or all")
		return
	}

	stats, err := s.db.GetContainerStats(containerID, hostID, hoursBack)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get container stats: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// handlePrometheusMetrics returns Prometheus-compatible metrics for all running containers
func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	containers, err := s.db.GetCurrentStatsForAllContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get container stats: "+err.Error())
		return
	}

	// Build Prometheus-format metrics
	var metrics strings.Builder

	// Write HELP and TYPE for each metric
	metrics.WriteString("# HELP census_container_cpu_percent Container CPU usage percentage\n")
	metrics.WriteString("# TYPE census_container_cpu_percent gauge\n")

	for _, c := range containers {
		if c.CPUPercent > 0 {
			metrics.WriteString(fmt.Sprintf(
				"census_container_cpu_percent{container_name=\"%s\",container_id=\"%s\",host_name=\"%s\",image=\"%s\"} %.2f\n",
				c.Name, c.ID[:12], c.HostName, c.Image, c.CPUPercent,
			))
		}
	}

	metrics.WriteString("\n# HELP census_container_memory_bytes Container memory usage in bytes\n")
	metrics.WriteString("# TYPE census_container_memory_bytes gauge\n")

	for _, c := range containers {
		if c.MemoryUsage > 0 {
			metrics.WriteString(fmt.Sprintf(
				"census_container_memory_bytes{container_name=\"%s\",container_id=\"%s\",host_name=\"%s\",image=\"%s\"} %d\n",
				c.Name, c.ID[:12], c.HostName, c.Image, c.MemoryUsage,
			))
		}
	}

	metrics.WriteString("\n# HELP census_container_memory_limit_bytes Container memory limit in bytes\n")
	metrics.WriteString("# TYPE census_container_memory_limit_bytes gauge\n")

	for _, c := range containers {
		if c.MemoryLimit > 0 {
			metrics.WriteString(fmt.Sprintf(
				"census_container_memory_limit_bytes{container_name=\"%s\",container_id=\"%s\",host_name=\"%s\",image=\"%s\"} %d\n",
				c.Name, c.ID[:12], c.HostName, c.Image, c.MemoryLimit,
			))
		}
	}

	// Write response with Prometheus content type
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(metrics.String()))
}

// handleGetChangesReport returns a comprehensive environment change report
func (s *Server) handleGetChangesReport(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	hostFilterStr := r.URL.Query().Get("host_id")

	// Default to last 7 days if not specified
	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid start time format (use RFC3339): "+err.Error())
			return
		}
	} else {
		start = time.Now().Add(-7 * 24 * time.Hour)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid end time format (use RFC3339): "+err.Error())
			return
		}
	} else {
		end = time.Now()
	}

	// Validate time range
	if end.Before(start) {
		respondError(w, http.StatusBadRequest, "End time must be after start time")
		return
	}

	var hostFilter int64
	if hostFilterStr != "" {
		hostFilter, err = strconv.ParseInt(hostFilterStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid host_id parameter: "+err.Error())
			return
		}
	}

	// Generate report
	report, err := s.db.GetChangesReport(start, end, hostFilter)
	if err != nil {
		log.Printf("Error generating changes report: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to generate report: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, report)
}
