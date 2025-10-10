package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// Submitter handles sending telemetry to multiple endpoints
type Submitter struct {
	config     models.TelemetryConfig
	httpClient *http.Client
}

// NewSubmitter creates a new telemetry submitter
func NewSubmitter(config models.TelemetryConfig) *Submitter {
	return &Submitter{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Submit sends telemetry report to all configured endpoints in parallel
func (s *Submitter) Submit(ctx context.Context, report *models.TelemetryReport) error {
	if !s.config.Enabled {
		return nil
	}

	// Filter enabled endpoints
	var endpoints []models.TelemetryEndpoint
	for _, ep := range s.config.Endpoints {
		if ep.Enabled && ep.URL != "" {
			endpoints = append(endpoints, ep)
		}
	}

	if len(endpoints) == 0 {
		log.Println("No telemetry endpoints configured")
		return nil
	}

	// Submit to all endpoints in parallel
	var wg sync.WaitGroup
	errors := make(chan error, len(endpoints))

	for _, endpoint := range endpoints {
		wg.Add(1)
		go func(ep models.TelemetryEndpoint) {
			defer wg.Done()
			if err := s.submitToEndpoint(ctx, ep, report); err != nil {
				log.Printf("Failed to submit telemetry to %s (%s): %v", ep.Name, ep.URL, err)
				errors <- fmt.Errorf("%s: %w", ep.Name, err)
			} else {
				log.Printf("Successfully submitted telemetry to %s (%s)", ep.Name, ep.URL)
			}
		}(endpoint)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to submit to %d/%d endpoints", len(errs), len(endpoints))
	}

	return nil
}

// submitToEndpoint sends report to a single endpoint with retry logic
func (s *Submitter) submitToEndpoint(ctx context.Context, endpoint models.TelemetryEndpoint, report *models.TelemetryReport) error {
	// Serialize report
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := time.Duration(1<<attempt) * 2 * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err = s.sendRequest(ctx, endpoint, data)
		if err == nil {
			return nil
		}

		log.Printf("Attempt %d/%d failed for %s: %v", attempt+1, maxRetries, endpoint.Name, err)
	}

	return fmt.Errorf("all %d attempts failed: %w", maxRetries, err)
}

// sendRequest performs the actual HTTP request
func (s *Submitter) sendRequest(ctx context.Context, endpoint models.TelemetryEndpoint, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "container-census-telemetry/1.0")

	// Add API key if configured
	if endpoint.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+endpoint.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
