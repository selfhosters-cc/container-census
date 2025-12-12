package api

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/selfhosters-cc/container-census/internal/models"
)

// UpdateCheckJob represents an in-flight bulk update check operation
type UpdateCheckJob struct {
	ID           string
	TotalCount   int
	CheckedCount int
	Results      map[string]*models.ImageUpdateInfo // key: "{host_id}-{container_id}"
	Status       string                             // "running", "complete", "error"
	Error        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	mu           sync.RWMutex
}

// UpdateJobManager manages in-memory update check jobs
type UpdateJobManager struct {
	jobs map[string]*UpdateCheckJob
	mu   sync.RWMutex
}

// NewUpdateJobManager creates a new job manager
func NewUpdateJobManager() *UpdateJobManager {
	return &UpdateJobManager{
		jobs: make(map[string]*UpdateCheckJob),
	}
}

// CreateJob creates a new update check job and returns its ID
func (m *UpdateJobManager) CreateJob(totalCount int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobID := uuid.New().String()
	job := &UpdateCheckJob{
		ID:           jobID,
		TotalCount:   totalCount,
		CheckedCount: 0,
		Results:      make(map[string]*models.ImageUpdateInfo),
		Status:       "running",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.jobs[jobID] = job
	return jobID
}

// GetJob retrieves a job by ID
func (m *UpdateJobManager) GetJob(jobID string) (*UpdateCheckJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, errors.New("job not found")
	}

	return job, nil
}

// UpdateProgress updates the progress of a job with a new result
func (m *UpdateJobManager) UpdateProgress(jobID string, containerKey string, result *models.ImageUpdateInfo) error {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("job not found")
	}

	job.mu.Lock()
	defer job.mu.Unlock()

	job.Results[containerKey] = result
	job.CheckedCount++
	job.UpdatedAt = time.Now()

	return nil
}

// CompleteJob marks a job as complete
func (m *UpdateJobManager) CompleteJob(jobID string) error {
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
func (m *UpdateJobManager) SetError(jobID string, err error) error {
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
func (m *UpdateJobManager) CleanupOldJobs() {
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
func (m *UpdateJobManager) GetJobSnapshot(jobID string) (map[string]interface{}, error) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()

	if !exists {
		return nil, errors.New("job not found")
	}

	job.mu.RLock()
	defer job.mu.RUnlock()

	snapshot := map[string]interface{}{
		"id":            job.ID,
		"total":         job.TotalCount,
		"checked":       job.CheckedCount,
		"status":        job.Status,
		"error":         job.Error,
		"created_at":    job.CreatedAt,
		"updated_at":    job.UpdatedAt,
		"results":       job.Results,
		"results_count": len(job.Results),
	}

	return snapshot, nil
}

// Stats returns current statistics about jobs
func (m *UpdateJobManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running := 0
	completed := 0
	errored := 0

	for _, job := range m.jobs {
		job.mu.RLock()
		status := job.Status
		job.mu.RUnlock()

		switch status {
		case "running":
			running++
		case "complete":
			completed++
		case "error":
			errored++
		}
	}

	return map[string]interface{}{
		"total_jobs": len(m.jobs),
		"running":    running,
		"completed":  completed,
		"errored":    errored,
	}
}

// LogProgress logs the current progress of a job (for debugging)
func (m *UpdateJobManager) LogProgress(jobID string) {
	snapshot, err := m.GetJobSnapshot(jobID)
	if err != nil {
		fmt.Printf("[UpdateJob] Error getting snapshot: %v\n", err)
		return
	}

	fmt.Printf("[UpdateJob] Job %s: %d/%d checked, status=%s\n",
		jobID[:8], snapshot["checked"], snapshot["total"], snapshot["status"])
}
