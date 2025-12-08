package security

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

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error JSON response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// handleGetSummary returns an overview of all vulnerability scans
func (p *SecurityPlugin) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := p.db.GetVulnerabilitySummary()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability summary: "+err.Error())
		return
	}

	// Add queue status if scheduler is available
	if p.vulnScheduler != nil {
		queueStatus := p.vulnScheduler.GetQueueStatus()
		response := map[string]interface{}{
			"summary":      summary,
			"queue_status": queueStatus,
		}
		respondJSON(w, http.StatusOK, response)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// handleGetImage returns vulnerabilities for a specific image
func (p *SecurityPlugin) handleGetImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	imageID := vars["imageId"]

	// Get scan metadata
	scan, err := p.db.GetVulnerabilityScan(imageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scan: "+err.Error())
		return
	}

	if scan == nil {
		respondError(w, http.StatusNotFound, "No scan found for image: "+imageID)
		return
	}

	// Get vulnerabilities
	vulns, err := p.db.GetVulnerabilities(imageID)
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

// handleGetContainer returns vulnerabilities for a specific container (via its image)
func (p *SecurityPlugin) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hostID, err := strconv.Atoi(vars["hostId"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}
	containerID := vars["containerId"]

	// Get container to find its image
	containers, err := p.db.GetLatestContainers()
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
	scan, err := p.db.GetVulnerabilityScan(container.ImageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scan: "+err.Error())
		return
	}

	if scan == nil {
		respondError(w, http.StatusNotFound, "No scan found for container image")
		return
	}

	// Get vulnerabilities
	vulns, err := p.db.GetVulnerabilities(container.ImageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerabilities: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"container_id":    container.ID,
		"container_name":  container.Name,
		"host_name":       container.HostName,
		"image_id":        container.ImageID,
		"image_name":      container.Image,
		"scan":            scan,
		"vulnerabilities": vulns,
	}

	respondJSON(w, http.StatusOK, result)
}

// handleTriggerScan queues an image for scanning
func (p *SecurityPlugin) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	if p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	vars := mux.Vars(r)
	imageID := vars["imageId"]

	// Try to get image name from database
	scans, err := p.db.GetAllVulnerabilityScans(1000)
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

	// Invalidate cache to force a fresh scan
	if p.vulnScanner != nil {
		p.vulnScanner.InvalidateCache(imageID)
	}

	// Force queue the scan with high priority (skip cache check)
	err = p.vulnScheduler.ForceQueueScan(imageID, imageName, 10)
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

// handleScanAll queues all images for rescanning
func (p *SecurityPlugin) handleScanAll(w http.ResponseWriter, r *http.Request) {
	if p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	// Get all unique images from recent scans
	scans, err := p.db.GetAllVulnerabilityScans(1000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get scans: "+err.Error())
		return
	}

	imageMap := make(map[string]string)
	for _, scan := range scans {
		imageMap[scan.ImageID] = scan.ImageName
	}

	count := p.vulnScheduler.RescanAll(imageMap)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Rescan triggered",
		"images_queued": count,
	})
}

// handleGetQueue returns the current scan queue status
func (p *SecurityPlugin) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	if p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	status := p.vulnScheduler.GetQueueStatus()
	respondJSON(w, http.StatusOK, status)
}

// handleUpdateDB triggers an update of the Trivy vulnerability database
func (p *SecurityPlugin) handleUpdateDB(w http.ResponseWriter, r *http.Request) {
	if p.vulnScanner == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	err := p.vulnScanner.UpdateTrivyDB(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update Trivy database: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Trivy database updated successfully",
	})
}

// handleGetSettings returns the current vulnerability scanner settings
func (p *SecurityPlugin) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if p.vulnScanner == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	config := p.vulnScanner.GetConfig()
	respondJSON(w, http.StatusOK, config)
}

// handleUpdateSettings updates the vulnerability scanner settings
func (p *SecurityPlugin) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if p.vulnScanner == nil || p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	var newConfig vulnerability.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate and update config
	currentConfig := p.vulnScanner.GetConfig()
	err := currentConfig.Update(&newConfig)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid configuration: "+err.Error())
		return
	}

	// Save to database
	err = p.db.SaveVulnerabilitySettings(currentConfig)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	// Update scanner and scheduler
	p.vulnScanner.SetConfig(currentConfig)
	p.vulnScheduler.UpdateConfig(currentConfig)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Settings updated successfully",
		"config":  currentConfig,
	})
}

// handleGetScans returns all vulnerability scans
func (p *SecurityPlugin) handleGetScans(w http.ResponseWriter, r *http.Request) {
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

	scans, err := p.db.GetAllVulnerabilityScans(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get vulnerability scans: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, scans)
}

// handleClear clears all vulnerability data
func (p *SecurityPlugin) handleClear(w http.ResponseWriter, r *http.Request) {
	// This endpoint clears all vulnerability scan data
	// Useful for testing or resetting the vulnerability database
	p.deps.Logger.Info("⚠️ DANGER ZONE: Clearing all vulnerability data...")

	// Use zero retention days to delete everything
	if err := p.db.CleanupOldVulnerabilityData(0, 0); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clear vulnerability data: "+err.Error())
		return
	}

	p.deps.Logger.Info("✅ All vulnerability data cleared")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All vulnerability scans and CVE data deleted",
	})
}
