package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/vulnerability"
	"github.com/gorilla/mux"
)

// VulnerabilityScanner interface for the vulnerability scanner
type VulnerabilityScanner interface {
	GetCachedScan(imageID string) (*vulnerability.VulnerabilityScan, error)
	ScanImage(ctx context.Context, imageRef string) (*vulnerability.VulnerabilityScanResult, error)
	UpdateTrivyDB(ctx context.Context) error
	GetConfig() *vulnerability.Config
	SetConfig(config *vulnerability.Config)
}

// VulnerabilityScheduler interface for the vulnerability scheduler
type VulnerabilityScheduler interface {
	QueueScan(imageID, imageName string, priority int) error
	QueueScanBlocking(imageID, imageName string, priority int) error
	GetQueueStatus() vulnerability.ScanQueueStatus
	RescanAll(imageIDs map[string]string) int
	UpdateConfig(config *vulnerability.Config)
}

// SetVulnerabilityScanner sets the vulnerability scanner and scheduler
func (s *Server) SetVulnerabilityScanner(scanner VulnerabilityScanner, scheduler VulnerabilityScheduler) {
	s.vulnScanner = scanner
	s.vulnScheduler = scheduler
}

// handleGetVulnerabilitySummary returns an overview of all vulnerability scans
func (s *Server) handleGetVulnerabilitySummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.db.GetVulnerabilitySummary()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability summary: "+err.Error())
		return
	}

	// Add queue status if scheduler is available
	if s.vulnScheduler != nil {
		queueStatus := s.vulnScheduler.GetQueueStatus()
		response := map[string]interface{}{
			"summary":      summary,
			"queue_status": queueStatus,
		}
		respondJSON(w, http.StatusOK, response)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// handleGetImageVulnerabilities returns vulnerabilities for a specific image
func (s *Server) handleGetImageVulnerabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imageID := vars["imageId"]

	// Get scan metadata
	scan, err := s.db.GetVulnerabilityScan(imageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scan: "+err.Error())
		return
	}

	if scan == nil {
		respondError(w, http.StatusNotFound, "No scan found for image: "+imageID)
		return
	}

	// Get vulnerabilities
	vulns, err := s.db.GetVulnerabilities(imageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerabilities: "+err.Error())
		return
	}

	result := vulnerability.VulnerabilityScanResult{
		Scan:            *scan,
		Vulnerabilities: vulns,
	}

	respondJSON(w, http.StatusOK, result)
}

// handleGetContainerVulnerabilities returns vulnerabilities for a specific container (via its image)
func (s *Server) handleGetContainerVulnerabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.Atoi(vars["hostId"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}
	containerID := vars["containerId"]

	// Get container to find its image
	containers, err := s.db.GetLatestContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers: "+err.Error())
		return
	}

	var container *models.Container
	for _, c := range containers {
		if c.ID == containerID && c.HostID == int64(hostID) {
			container = &c
			break
		}
	}

	if container == nil {
		respondError(w, http.StatusNotFound, "Container not found")
		return
	}

	// Get scan for the container's image
	scan, err := s.db.GetVulnerabilityScan(container.ImageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scan: "+err.Error())
		return
	}

	if scan == nil {
		respondError(w, http.StatusNotFound, "No scan found for container image")
		return
	}

	// Get vulnerabilities
	vulns, err := s.db.GetVulnerabilities(container.ImageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerabilities: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"container_id":   container.ID,
		"container_name": container.Name,
		"host_name":      container.HostName,
		"image_id":       container.ImageID,
		"image_name":     container.Image,
		"scan":           scan,
		"vulnerabilities": vulns,
	}

	respondJSON(w, http.StatusOK, result)
}

// handleTriggerImageScan queues an image for scanning
func (s *Server) handleTriggerImageScan(w http.ResponseWriter, r *http.Request) {
	if s.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	vars := mux.Vars(r)
	imageID := vars["imageId"]

	// Try to get image name from database
	scans, err := s.db.GetAllVulnerabilityScans(1000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query scans: "+err.Error())
		return
	}

	imageName := imageID // fallback to ID
	for _, scan := range scans {
		if scan.ImageID == imageID {
			imageName = scan.ImageName
			break
		}
	}

	// Queue the scan with high priority
	err = s.vulnScheduler.QueueScan(imageID, imageName, 10)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to queue scan: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":                "Scan queued",
		"image_id":               imageID,
		"estimated_time_seconds": 30,
	})
}

// handleTriggerScanAll queues all images for rescanning
func (s *Server) handleTriggerScanAll(w http.ResponseWriter, r *http.Request) {
	if s.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	// Get all unique images from recent scans
	scans, err := s.db.GetAllVulnerabilityScans(1000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get scans: "+err.Error())
		return
	}

	imageMap := make(map[string]string)
	for _, scan := range scans {
		imageMap[scan.ImageID] = scan.ImageName
	}

	count := s.vulnScheduler.RescanAll(imageMap)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Rescan triggered",
		"images_queued": count,
	})
}

// handleGetScanQueue returns the current scan queue status
func (s *Server) handleGetScanQueue(w http.ResponseWriter, r *http.Request) {
	if s.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	status := s.vulnScheduler.GetQueueStatus()
	respondJSON(w, http.StatusOK, status)
}

// handleUpdateTrivyDB triggers an update of the Trivy vulnerability database
func (s *Server) handleUpdateTrivyDB(w http.ResponseWriter, r *http.Request) {
	if s.vulnScanner == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	err := s.vulnScanner.UpdateTrivyDB(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update Trivy database: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Trivy database updated successfully",
	})
}

// handleGetVulnerabilitySettings returns the current vulnerability scanner settings
func (s *Server) handleGetVulnerabilitySettings(w http.ResponseWriter, r *http.Request) {
	if s.vulnScanner == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	config := s.vulnScanner.GetConfig()
	respondJSON(w, http.StatusOK, config)
}

// handleUpdateVulnerabilitySettings updates the vulnerability scanner settings
func (s *Server) handleUpdateVulnerabilitySettings(w http.ResponseWriter, r *http.Request) {
	if s.vulnScanner == nil || s.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	var newConfig vulnerability.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate and update config
	currentConfig := s.vulnScanner.GetConfig()
	err := currentConfig.Update(&newConfig)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid configuration: "+err.Error())
		return
	}

	// Save to database
	err = s.db.SaveVulnerabilitySettings(currentConfig)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	// Update scanner and scheduler
	s.vulnScanner.SetConfig(currentConfig)
	s.vulnScheduler.UpdateConfig(currentConfig)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Settings updated successfully",
		"config":  currentConfig,
	})
}

// handleGetAllVulnerabilityScans returns all vulnerability scans
func (s *Server) handleGetAllVulnerabilityScans(w http.ResponseWriter, r *http.Request) {
	// Get limit from query params (default 100)
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 100
		}
	}

	scans, err := s.db.GetAllVulnerabilityScans(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scans: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, scans)
}
