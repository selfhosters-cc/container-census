package api

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LayerProgress represents the progress of a single image layer
type LayerProgress struct {
	ID      string `json:"id"`
	Status  string `json:"status"`  // "waiting", "downloading", "extracting", "complete"
	Current int64  `json:"current"` // bytes downloaded/extracted
	Total   int64  `json:"total"`   // total bytes
}

// PullJob represents an in-flight image pull operation
type PullJob struct {
	ID          string                    `json:"id"`
	ImageName   string                    `json:"image_name"`
	HostID      int64                     `json:"host_id"`
	HostName    string                    `json:"host_name"`
	ContainerID string                    `json:"container_id,omitempty"`
	Status      string                    `json:"status"` // "pulling", "complete", "error"
	Error       string                    `json:"error,omitempty"`
	Layers      map[string]*LayerProgress `json:"layers"`
	Message     string                    `json:"message,omitempty"` // status message
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	mu          sync.RWMutex
}

// PullJobManager manages in-memory pull jobs
type PullJobManager struct {
	jobs map[string]*PullJob
	mu   sync.RWMutex
}

// NewPullJobManager creates a new pull job manager
func NewPullJobManager() *PullJobManager {
	return &PullJobManager{
		jobs: make(map[string]*PullJob),
	}
}

// CreateJob creates a new pull job and returns its ID
func (m *PullJobManager) CreateJob(imageName string, hostID int64, hostName string, containerID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := uuid.New().String()
	job := &PullJob{
		ID:          jobID,
		ImageName:   imageName,
		HostID:      hostID,
		HostName:    hostName,
		ContainerID: containerID,
		Status:      "pulling",
		Layers:      make(map[string]*LayerProgress),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.jobs[jobID] = job
	return jobID
}

// GetJob retrieves a job by ID
func (m *PullJobManager) GetJob(jobID string) (*PullJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, errors.New("job not found")
	}

	return job, nil
}

// UpdateLayerProgress updates the progress of a specific layer
func (m *PullJobManager) UpdateLayerProgress(jobID string, layerID string, status string, current, total int64) error {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("job not found")
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	if job.Layers[layerID] == nil {
		job.Layers[layerID] = &LayerProgress{
			ID: layerID,
		}
	}

	layer := job.Layers[layerID]
	layer.Status = status
	if current > 0 {
		layer.Current = current
	}
	if total > 0 {
		layer.Total = total
	}
	job.UpdatedAt = time.Now()

	return nil
}

// SetMessage sets a status message on the job
func (m *PullJobManager) SetMessage(jobID string, message string) error {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("job not found")
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	job.Message = message
	job.UpdatedAt = time.Now()

	return nil
}

// CompleteJob marks a job as complete
func (m *PullJobManager) CompleteJob(jobID string) error {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("job not found")
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	job.Status = "complete"
	job.UpdatedAt = time.Now()

	return nil
}

// SetError marks a job as failed with an error message
func (m *PullJobManager) SetError(jobID string, err error) error {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("job not found")
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	job.Status = "error"
	job.Error = err.Error()
	job.UpdatedAt = time.Now()

	return nil
}

// CleanupOldJobs removes jobs older than the specified TTL
func (m *PullJobManager) CleanupOldJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	ttl := 1 * time.Hour
	cutoff := time.Now().Add(-ttl)

	for jobID, job := range m.jobs {
		job.mu.RLock()
		createdAt := job.CreatedAt
		job.mu.RUnlock()

		if createdAt.Before(cutoff) {
			delete(m.jobs, jobID)
		}
	}
}

// GetJobSnapshot returns a thread-safe snapshot of the job's current state
func (m *PullJobManager) GetJobSnapshot(jobID string) (map[string]interface{}, error) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return nil, errors.New("job not found")
	}

	job.mu.RLock()
	defer job.mu.RUnlock()

	// Copy layers map
	layers := make(map[string]*LayerProgress)
	for k, v := range job.Layers {
		layers[k] = &LayerProgress{
			ID:      v.ID,
			Status:  v.Status,
			Current: v.Current,
			Total:   v.Total,
		}
	}

	// Calculate overall progress
	var totalBytes, downloadedBytes int64
	var completedLayers, totalLayers int
	for _, layer := range job.Layers {
		totalLayers++
		if layer.Total > 0 {
			totalBytes += layer.Total
			downloadedBytes += layer.Current
		}
		if layer.Status == "complete" || layer.Status == "Pull complete" || layer.Status == "Already exists" {
			completedLayers++
		}
	}

	var overallPercent float64
	if totalBytes > 0 {
		overallPercent = float64(downloadedBytes) / float64(totalBytes) * 100
	}

	snapshot := map[string]interface{}{
		"id":               job.ID,
		"image_name":       job.ImageName,
		"host_id":          job.HostID,
		"host_name":        job.HostName,
		"container_id":     job.ContainerID,
		"status":           job.Status,
		"error":            job.Error,
		"message":          job.Message,
		"layers":           layers,
		"layer_count":      totalLayers,
		"completed_layers": completedLayers,
		"total_bytes":      totalBytes,
		"downloaded_bytes": downloadedBytes,
		"overall_percent":  overallPercent,
		"created_at":       job.CreatedAt,
		"updated_at":       job.UpdatedAt,
	}

	return snapshot, nil
}

// Stats returns current statistics about pull jobs
func (m *PullJobManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pulling := 0
	completed := 0
	errored := 0

	for _, job := range m.jobs {
		job.mu.RLock()
		status := job.Status
		job.mu.RUnlock()

		switch status {
		case "pulling":
			pulling++
		case "complete":
			completed++
		case "error":
			errored++
		}
	}

	return map[string]interface{}{
		"total_jobs": len(m.jobs),
		"pulling":    pulling,
		"completed":  completed,
		"errored":    errored,
	}
}
