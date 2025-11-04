package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
)

// mockDB implements the database interface needed by Submitter
type mockDB struct {
	mu                sync.Mutex
	successCalls      map[string]int
	failureCalls      map[string]int
	lastFailureReason map[string]string
	lastFailureTime   map[string]time.Time
}

func newMockDB() *mockDB {
	return &mockDB{
		successCalls:      make(map[string]int),
		failureCalls:      make(map[string]int),
		lastFailureReason: make(map[string]string),
		lastFailureTime:   make(map[string]time.Time),
	}
}

func (m *mockDB) SaveTelemetrySuccess(endpointName, endpointURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successCalls[endpointName]++
	return nil
}

func (m *mockDB) SaveTelemetryFailure(endpointName, endpointURL, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureCalls[endpointName]++
	m.lastFailureReason[endpointName] = reason
	m.lastFailureTime[endpointName] = time.Now()
	return nil
}

func (m *mockDB) SaveTelemetrySubmission(submission *models.TelemetrySubmission) error {
	// Mock implementation - just return nil for tests
	return nil
}

func (m *mockDB) GetTelemetryStatus(endpointName string) (*models.TelemetryEndpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return status with last failure time if endpoint has failed
	if failureTime, exists := m.lastFailureTime[endpointName]; exists {
		return &models.TelemetryEndpoint{
			Name:        endpointName,
			LastFailure: &failureTime,
		}, nil
	}

	// Return nil to indicate no status (allows endpoint to be tried)
	return nil, nil
}

func (m *mockDB) getSuccessCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.successCalls[name]
}

func (m *mockDB) getFailureCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failureCalls[name]
}

// TestSubmitPrivateEnabledCommunityDisabled tests submission with only private collector enabled
func TestSubmitPrivateEnabledCommunityDisabled(t *testing.T) {
	// Create test servers
	privateCalls := 0
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer privateServer.Close()

	communityCalls := 0
	communityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		communityCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer communityServer.Close()

	// Create config with private enabled, community disabled
	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "community",
				URL:     communityServer.URL,
			},
			{
				Name:    "private",
				URL:     privateServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only private collector received the telemetry
	if privateCalls != 1 {
		t.Errorf("Expected 1 call to private collector, got %d", privateCalls)
	}

	if communityCalls != 0 {
		t.Errorf("Expected 0 calls to community collector (disabled), got %d", communityCalls)
	}

	// Verify database records
	if db.getSuccessCount("private") != 1 {
		t.Errorf("Expected 1 success record for private, got %d", db.getSuccessCount("private"))
	}

	if db.getSuccessCount("community") != 0 {
		t.Errorf("Expected 0 success records for community (disabled), got %d", db.getSuccessCount("community"))
	}
}

// TestSubmitCommunityEnabledPrivateDisabled tests submission with only community collector enabled
func TestSubmitCommunityEnabledPrivateDisabled(t *testing.T) {
	// Create test servers
	privateCalls := 0
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer privateServer.Close()

	communityCalls := 0
	communityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		communityCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer communityServer.Close()

	// Create config with community enabled, private disabled
	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "community",
				URL:     communityServer.URL,
			},
			{
				Name:    "private",
				URL:     privateServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only community collector received the telemetry
	if communityCalls != 1 {
		t.Errorf("Expected 1 call to community collector, got %d", communityCalls)
	}

	if privateCalls != 0 {
		t.Errorf("Expected 0 calls to private collector (disabled), got %d", privateCalls)
	}

	// Verify database records
	if db.getSuccessCount("community") != 1 {
		t.Errorf("Expected 1 success record for community, got %d", db.getSuccessCount("community"))
	}

	if db.getSuccessCount("private") != 0 {
		t.Errorf("Expected 0 success records for private (disabled), got %d", db.getSuccessCount("private"))
	}
}

// TestSubmitBothEnabled tests submission with both collectors enabled
func TestSubmitBothEnabled(t *testing.T) {
	// Create test servers
	privateCalls := 0
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateCalls++

		// Verify the request body contains valid telemetry
		var report models.TelemetryReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Errorf("Failed to decode telemetry report: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer privateServer.Close()

	communityCalls := 0
	communityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		communityCalls++

		// Verify the request body contains valid telemetry
		var report models.TelemetryReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Errorf("Failed to decode telemetry report: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer communityServer.Close()

	// Create config with both enabled
	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "community",
				URL:     communityServer.URL,
			},
			{
				Name:    "private",
				URL:     privateServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify both collectors received the telemetry
	if communityCalls != 1 {
		t.Errorf("Expected 1 call to community collector, got %d", communityCalls)
	}

	if privateCalls != 1 {
		t.Errorf("Expected 1 call to private collector, got %d", privateCalls)
	}

	// Verify database records for both
	if db.getSuccessCount("community") != 1 {
		t.Errorf("Expected 1 success record for community, got %d", db.getSuccessCount("community"))
	}

	if db.getSuccessCount("private") != 1 {
		t.Errorf("Expected 1 success record for private, got %d", db.getSuccessCount("private"))
	}
}

// TestSubmitBothDisabled tests submission with both collectors disabled
func TestSubmitBothDisabled(t *testing.T) {
	// Create test servers
	privateCalls := 0
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer privateServer.Close()

	communityCalls := 0
	communityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		communityCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer communityServer.Close()

	// Create config with both disabled
	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "community",
				URL:     communityServer.URL,
			},
			{
				Name:    "private",
				URL:     privateServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify neither collector received telemetry
	if communityCalls != 0 {
		t.Errorf("Expected 0 calls to community collector (disabled), got %d", communityCalls)
	}

	if privateCalls != 0 {
		t.Errorf("Expected 0 calls to private collector (disabled), got %d", privateCalls)
	}

	// Verify no database records
	if db.getSuccessCount("community") != 0 {
		t.Errorf("Expected 0 success records for community (disabled), got %d", db.getSuccessCount("community"))
	}

	if db.getSuccessCount("private") != 0 {
		t.Errorf("Expected 0 success records for private (disabled), got %d", db.getSuccessCount("private"))
	}
}

// TestSubmitTelemetryGloballyDisabled tests that nothing happens when telemetry is globally disabled
func TestSubmitTelemetryGloballyDisabled(t *testing.T) {
	// Create test servers
	privateCalls := 0
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer privateServer.Close()

	communityCalls := 0
	communityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		communityCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer communityServer.Close()

	// Create config with telemetry globally disabled
	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "community",
				URL:     communityServer.URL,
			},
			{
				Name:    "private",
				URL:     privateServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify neither collector received telemetry (global flag disabled)
	if communityCalls != 0 {
		t.Errorf("Expected 0 calls to community collector (globally disabled), got %d", communityCalls)
	}

	if privateCalls != 0 {
		t.Errorf("Expected 0 calls to private collector (globally disabled), got %d", privateCalls)
	}

	// Verify no database records
	if db.getSuccessCount("community") != 0 {
		t.Errorf("Expected 0 success records for community (globally disabled), got %d", db.getSuccessCount("community"))
	}

	if db.getSuccessCount("private") != 0 {
		t.Errorf("Expected 0 success records for private (globally disabled), got %d", db.getSuccessCount("private"))
	}
}

// TestSubmitWithFailure tests that failures are recorded correctly
func TestSubmitWithFailure(t *testing.T) {
	// Create a server that returns error
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	// Create a successful server
	successCalls := 0
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer successServer.Close()

	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "failing",
				URL:     failingServer.URL,
			},
			{
				Name:    "working",
				URL:     successServer.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	// Should return error since one endpoint failed
	if err == nil {
		t.Error("Expected error when one endpoint fails")
	}

	// Verify success server was called
	if successCalls != 1 {
		t.Errorf("Expected 1 call to working collector, got %d", successCalls)
	}

	// Verify database records
	if db.getSuccessCount("working") != 1 {
		t.Errorf("Expected 1 success record for working endpoint, got %d", db.getSuccessCount("working"))
	}

	if db.getFailureCount("failing") != 1 {
		t.Errorf("Expected 1 failure record for failing endpoint, got %d", db.getFailureCount("failing"))
	}

	// Verify failure reason was recorded
	if reason := db.lastFailureReason["failing"]; reason == "" {
		t.Error("Expected failure reason to be recorded")
	}
}

// TestSubmitWithEmptyURL tests that endpoints with empty URLs are skipped
func TestSubmitWithEmptyURL(t *testing.T) {
	// Create test server
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "empty-url",
				URL:     "", // EMPTY URL
			},
			{
				Name:    "valid",
				URL:     server.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()
	err := submitter.Submit(ctx, report)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify only the valid endpoint was called
	if calls != 1 {
		t.Errorf("Expected 1 call to valid endpoint, got %d", calls)
	}

	// Verify database records
	if db.getSuccessCount("valid") != 1 {
		t.Errorf("Expected 1 success record for valid endpoint, got %d", db.getSuccessCount("valid"))
	}

	if db.getSuccessCount("empty-url") != 0 {
		t.Errorf("Expected 0 success records for empty-url endpoint, got %d", db.getSuccessCount("empty-url"))
	}
}

// TestCircuitBreaker tests that recently-failed endpoints are skipped
func TestCircuitBreaker(t *testing.T) {
	// Create test server
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError) // Always fail
	}))
	defer server.Close()

	config := models.TelemetryConfig{
		IntervalHours: 24,
		Endpoints: []models.TelemetryEndpoint{
			{
				Name:    "failing",
				URL:     server.URL,
			},
		},
	}

	db := newMockDB()
	submitter := NewSubmitter(config, db)
	// Set circuit breaker to very short duration for testing
	submitter.circuitBreakerWindow = 100 * time.Millisecond

	report := &models.TelemetryReport{
		InstallationID:  "test-install",
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		HostCount:       1,
		TotalContainers: 5,
	}

	ctx := context.Background()

	// First submission - should fail and record the failure
	err := submitter.Submit(ctx, report)
	if err == nil {
		t.Error("Expected error on first submission")
	}

	firstCallCount := calls

	// Second submission immediately after - should skip due to circuit breaker
	err = submitter.Submit(ctx, report)
	if err != nil {
		t.Errorf("Expected no error when circuit breaker skips all endpoints, got: %v", err)
	}

	// Verify the endpoint was NOT called again (circuit breaker active)
	if calls != firstCallCount {
		t.Errorf("Expected circuit breaker to prevent retry, but endpoint was called again (calls: %d)", calls)
	}

	// Wait for circuit breaker window to expire
	time.Sleep(150 * time.Millisecond)

	// Third submission after window - should retry
	err = submitter.Submit(ctx, report)
	if err == nil {
		t.Error("Expected error after circuit breaker expired")
	}

	// Verify the endpoint WAS called again (circuit breaker expired)
	if calls == firstCallCount {
		t.Error("Expected circuit breaker to expire and allow retry")
	}
}
