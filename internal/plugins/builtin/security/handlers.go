package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/selfhosters-cc/container-census/internal/models"
	"github.com/selfhosters-cc/container-census/internal/vulnerability"
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

// handleScanAll queues all images for rescanning across selected hosts
func (p *SecurityPlugin) handleScanAll(w http.ResponseWriter, r *http.Request) {
	if p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	// Try to parse request body for host IDs
	var req struct {
		HostIDs []int64 `json:"host_ids"` // Optional: if empty, scan all enabled hosts
	}

	// Parse body if present (ignore errors for backward compatibility)
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	// Get all hosts
	hosts, err := p.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	// Determine which hosts to scan
	hostsToScan := make(map[int64]models.Host)
	if len(req.HostIDs) > 0 {
		// Scan only selected hosts
		for _, hostID := range req.HostIDs {
			// Find and include this host
			for _, host := range hosts {
				if host.ID == hostID {
					hostsToScan[hostID] = host
					break
				}
			}
		}
	} else {
		// Scan all hosts (default behavior)
		for _, host := range hosts {
			hostsToScan[host.ID] = host
		}
	}

	queuedByHost := make(map[string]int)
	totalQueued := 0

	// Get all containers
	localContainers, err := p.db.GetLatestContainers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get containers: "+err.Error())
		return
	}

	// Scan each selected host
	for hostID, host := range hostsToScan {
		hostCount := 0
		imagesSeen := make(map[string]bool)

		// Get containers for this host
		for _, container := range localContainers {
			if container.HostID == hostID && !imagesSeen[container.ImageID] {
				imagesSeen[container.ImageID] = true
				// Use force=true to bypass cache - user explicitly requested scan
				if err := p.vulnScheduler.QueueScanWithHostForce(container.ImageID, container.Image, hostID, 5); err == nil {
					hostCount++
				}
			}
		}

		if hostCount > 0 {
			hostName := host.Name
			if hostName == "" {
				hostName = "Local"
			}
			queuedByHost[hostName] = hostCount
			totalQueued += hostCount
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":         "Multi-host scan triggered",
		"queued_by_host":  queuedByHost,
		"total_queued":    totalQueued,
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

// handleGetProgress returns detailed scan progress information
func (p *SecurityPlugin) handleGetProgress(w http.ResponseWriter, r *http.Request) {
	if p.vulnScheduler == nil {
		respondError(w, http.StatusServiceUnavailable, "Vulnerability scanner not available")
		return
	}

	status := p.vulnScheduler.GetQueueStatus()

	// Transform queue items into current scans format
	currentScans := make([]map[string]interface{}, 0)
	for _, item := range status.QueueItems {
		// Only include items that are likely being processed (first N items where N = active workers)
		if len(currentScans) < status.ActiveWorkers {
			currentScans = append(currentScans, map[string]interface{}{
				"image_id":   item.ImageID,
				"image_name": item.ImageName,
				"host_id":    item.HostID,
				"host_name":  item.HostName,
				"started_at": item.QueuedAt,
			})
		}
	}

	response := map[string]interface{}{
		"in_progress":   status.InProgress,
		"pending":       status.Queued,
		"total":         status.InProgress + status.Queued,
		"current_scans": currentScans,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetTrivyStatus returns Trivy status for all hosts
func (p *SecurityPlugin) handleGetTrivyStatus(w http.ResponseWriter, r *http.Request) {
	// Get all hosts
	hosts, err := p.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	type TrivyHostStatus struct {
		HostID       int64     `json:"host_id"`
		HostName     string    `json:"host_name"`
		TrivyVersion string    `json:"trivy_version"`
		DBVersion    string    `json:"db_version"`
		LastUpdated  time.Time `json:"last_updated"`
		HasTrivy     bool      `json:"has_trivy"`
	}

	var hostStatuses []TrivyHostStatus

	// Check each host (including unix and agent types)
	for _, host := range hosts {
		status := TrivyHostStatus{
			HostID:   host.ID,
			HostName: host.Name,
			HasTrivy: false,
		}

		// Get metadata from database
		if metadata, err := p.db.GetTrivyDBMetadata(host.ID); err == nil && metadata != nil {
			status.TrivyVersion = metadata.TrivyVersion
			status.DBVersion = metadata.DBVersion
			status.LastUpdated = metadata.LastUpdated
		}

		// Determine if host has Trivy based on type
		if host.HostType == "unix" {
			// Local unix socket - has Trivy if scanner is available
			status.HasTrivy = p.vulnScanner != nil
		} else if host.HostType == "agent" && p.vulnScanner != nil {
			// Agent hosts - query the /info endpoint to check Trivy capability
			if agentInfo, err := p.vulnScanner.GetAgentInfo(r.Context(), &host); err == nil {
				status.HasTrivy = agentInfo.HasTrivy
				if agentInfo.TrivyVersion != "" {
					status.TrivyVersion = agentInfo.TrivyVersion
				}
			}
		}

		hostStatuses = append(hostStatuses, status)
	}

	response := map[string]interface{}{
		"hosts": hostStatuses,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleUpdateDB triggers an update of the Trivy vulnerability database
func (p *SecurityPlugin) handleUpdateDB(w http.ResponseWriter, r *http.Request) {
	// Parse request body for host IDs (optional)
	type UpdateRequest struct {
		HostIDs []int64 `json:"host_ids"`
	}

	var req UpdateRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	// Get all hosts
	hosts, err := p.db.GetHosts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get hosts: "+err.Error())
		return
	}

	// If no host IDs specified, update local unix hosts only (default behavior)
	if len(req.HostIDs) == 0 {
		for _, h := range hosts {
			if h.HostType == "unix" {
				req.HostIDs = append(req.HostIDs, h.ID)
			}
		}
	}

	type UpdateResult struct {
		HostID   int64  `json:"host_id"`
		HostName string `json:"host_name"`
		Success  bool   `json:"success"`
		Error    string `json:"error,omitempty"`
	}

	var results []UpdateResult
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	// Build host map for lookups
	hostMap := make(map[int64]*models.Host)
	for i := range hosts {
		hostMap[hosts[i].ID] = &hosts[i]
	}

	for _, hostID := range req.HostIDs {
		targetHost := hostMap[hostID]

		result := UpdateResult{
			HostID:   hostID,
			HostName: "Unknown",
		}

		if targetHost != nil {
			result.HostName = targetHost.Name
		}

		if targetHost == nil {
			result.Success = false
			result.Error = "Host not found"
		} else if targetHost.HostType == "unix" {
			// Update local Trivy DB (unix socket host)
			if p.vulnScanner == nil {
				result.Success = false
				result.Error = "Local vulnerability scanner not available"
			} else if err := p.vulnScanner.UpdateTrivyDB(ctx); err != nil {
				result.Success = false
				result.Error = err.Error()
			} else {
				result.Success = true
				// Save metadata
				p.db.SaveTrivyDBMetadata(targetHost.ID, "", "")
			}
		} else if targetHost.HostType == "agent" {
			// Update agent Trivy DB
			updateErr := updateAgentTrivyDB(ctx, *targetHost)
			if updateErr != nil {
				result.Success = false
				result.Error = updateErr.Error()
			} else {
				result.Success = true
				// Save metadata to track last update time
				p.db.SaveTrivyDBMetadata(targetHost.ID, "", "")
			}
		} else {
			result.Success = false
			result.Error = "Not a unix or agent host"
		}

		results = append(results, result)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
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

// updateAgentTrivyDB calls the agent's Trivy DB update endpoint
func updateAgentTrivyDB(ctx context.Context, host models.Host) error {
	// Build agent URL
	agentURL := host.Address
	if !strings.HasPrefix(agentURL, "http://") && !strings.HasPrefix(agentURL, "https://") {
		agentURL = "http://" + agentURL
	}
	agentURL = strings.TrimSuffix(agentURL, "/") + "/api/vulnerabilities/db-update"

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", agentURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add auth token
	if host.AgentToken != "" {
		req.Header.Set("X-API-Token", host.AgentToken)
	}

	// Send request
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call agent: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
