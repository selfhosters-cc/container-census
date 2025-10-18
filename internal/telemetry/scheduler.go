package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/scanner"
	"github.com/container-census/container-census/internal/storage"
)

// Scheduler handles periodic telemetry collection and submission
type Scheduler struct {
	collector  *Collector
	submitter  *Submitter
	scanner    *scanner.Scanner
	db         *storage.DB
	config     models.TelemetryConfig
	stopChan   chan struct{}
}

// NewScheduler creates a new telemetry scheduler
func NewScheduler(db *storage.DB, scanner *scanner.Scanner, config models.TelemetryConfig, scanInterval int) (*Scheduler, error) {
	collector, err := NewCollector(db, scanInterval)
	if err != nil {
		return nil, err
	}

	submitter := NewSubmitter(config, db)

	return &Scheduler{
		collector: collector,
		submitter: submitter,
		scanner:   scanner,
		db:        db,
		config:    config,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins the periodic telemetry collection
func (s *Scheduler) Start(ctx context.Context) {
	if !s.config.Enabled {
		log.Println("Telemetry is disabled")
		return
	}

	if s.config.IntervalHours <= 0 {
		log.Println("Invalid telemetry interval, defaulting to 168 hours (weekly)")
		s.config.IntervalHours = 168
	}

	interval := time.Duration(s.config.IntervalHours) * time.Hour
	log.Printf("Telemetry enabled: reporting every %v to %d endpoint(s)", interval, len(s.config.Endpoints))

	// Submit initial report after 5 minutes (to allow system to stabilize)
	time.AfterFunc(5*time.Minute, func() {
		s.collectAndSubmit(ctx)
	})

	// Start periodic ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectAndSubmit(ctx)
		case <-s.stopChan:
			log.Println("Telemetry scheduler stopped")
			return
		case <-ctx.Done():
			log.Println("Telemetry scheduler context cancelled")
			return
		}
	}
}

// Stop stops the telemetry scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// collectAndSubmit gathers telemetry and sends to all endpoints
func (s *Scheduler) collectAndSubmit(ctx context.Context) {
	log.Println("Collecting telemetry data...")

	// Collect agent stats from all agents
	agentStats := s.collectAgentStats(ctx)

	// Generate report
	report, err := s.collector.CollectReport(ctx, agentStats)
	if err != nil {
		log.Printf("Failed to collect telemetry report: %v", err)
		return
	}

	log.Printf("Telemetry collected: %d hosts, %d agents, %d containers, %d unique images",
		report.HostCount, report.AgentCount, report.TotalContainers, len(report.ImageStats))

	// Submit to all endpoints
	if err := s.submitter.Submit(ctx, report); err != nil {
		log.Printf("Telemetry submission had errors: %v", err)
	}
}

// SubmitNow triggers an immediate telemetry collection and submission
func (s *Scheduler) SubmitNow(ctx context.Context) error {
	if !s.config.Enabled {
		return fmt.Errorf("telemetry is not enabled")
	}

	log.Println("Manual telemetry submission triggered")
	s.collectAndSubmit(ctx)
	return nil
}

// collectAgentStats fetches telemetry data from all agents
func (s *Scheduler) collectAgentStats(ctx context.Context) map[string]*models.AgentInfo {
	hosts, err := s.db.GetHosts()
	if err != nil {
		log.Printf("Failed to get hosts for agent stats: %v", err)
		return nil
	}

	agentStats := make(map[string]*models.AgentInfo)

	for _, host := range hosts {
		if !host.Enabled || host.HostType != "agent" {
			continue
		}

		// Get agent info
		info, err := s.scanner.GetAgentInfo(ctx, host)
		if err != nil {
			log.Printf("Failed to get agent info for %s: %v", host.Name, err)
			continue
		}

		agentStats[host.Name] = info
	}

	return agentStats
}
