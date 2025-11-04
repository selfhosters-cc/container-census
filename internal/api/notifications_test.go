package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/container-census/container-census/internal/models"
	"github.com/container-census/container-census/internal/storage"
	"github.com/gorilla/mux"
)

func setupTestServer(t *testing.T) (*Server, *storage.DB) {
	t.Helper()

	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "census-api-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	tmpfile.Close()

	// Clean up on test completion
	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
	})

	db, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	server := &Server{
		db:     db,
		router: mux.NewRouter(),
	}

	return server, db
}

// TestCreateSilenceWithHTMLDatetime tests creating a silence with HTML datetime-local format
func TestCreateSilenceWithHTMLDatetime(t *testing.T) {
	server, db := setupTestServer(t)

	// Create a host first
	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	// Test cases with different datetime formats
	testCases := []struct {
		name           string
		silencedUntil  string
		expectSuccess  bool
		errorContains  string
	}{
		{
			name:          "HTML datetime-local format",
			silencedUntil: "2026-11-04T14:06",
			expectSuccess: true,
		},
		{
			name:          "HTML datetime-local with seconds",
			silencedUntil: "2026-11-04T14:06:00",
			expectSuccess: true,
		},
		{
			name:          "RFC3339 format",
			silencedUntil: "2026-11-04T14:06:00Z",
			expectSuccess: true,
		},
		{
			name:          "RFC3339 with timezone",
			silencedUntil: "2026-11-04T14:06:00-05:00",
			expectSuccess: true,
		},
		{
			name:          "Invalid format",
			silencedUntil: "not-a-date",
			expectSuccess: false,
			errorContains: "Invalid silenced_until format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"host_id":           host.ID,
				"container_pattern": "test-*",
				"silenced_until":    tc.silencedUntil,
				"reason":            tc.name,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest("POST", "/api/notifications/silences", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.handleCreateNotificationSilence(rec, req)

			if tc.expectSuccess {
				if rec.Code != http.StatusCreated {
					t.Errorf("Expected status 201, got %d. Body: %s", rec.Code, rec.Body.String())
				}

				var silence models.NotificationSilence
				if err := json.NewDecoder(rec.Body).Decode(&silence); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if silence.ID == 0 {
					t.Error("Expected silence ID to be set")
				}

				if silence.Reason != tc.name {
					t.Errorf("Expected reason %s, got %s", tc.name, silence.Reason)
				}

				// Verify it was saved to database
				active, err := db.GetActiveSilences()
				if err != nil {
					t.Fatalf("GetActiveSilences failed: %v", err)
				}

				found := false
				for _, s := range active {
					if s.ID == silence.ID {
						found = true
						break
					}
				}

				if !found {
					t.Error("Silence not found in database")
				}
			} else {
				if rec.Code != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", rec.Code)
				}

				var errResp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if errResp["error"] == "" {
					t.Error("Expected error message in response")
				}

				if tc.errorContains != "" && !contains(errResp["error"], tc.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.errorContains, errResp["error"])
				}
			}
		})
	}
}

// TestGetActiveSilencesEmptyArray tests that empty silences return [] not null
func TestGetActiveSilencesEmptyArray(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/notifications/silences", nil)
	rec := httptest.NewRecorder()

	server.handleGetNotificationSilences(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Check that response is [] not null
	body := rec.Body.String()
	if body != "[]\n" && body != "[]" {
		t.Errorf("Expected empty array [], got: %s", body)
	}

	var silences []models.NotificationSilence
	if err := json.NewDecoder(rec.Body).Decode(&silences); err != nil {
		// Body was already read, need to create new reader
		if err := json.Unmarshal([]byte(body), &silences); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
	}

	if silences == nil {
		t.Error("Expected empty slice, not nil")
	}

	if len(silences) != 0 {
		t.Errorf("Expected 0 silences, got %d", len(silences))
	}
}

// TestGetActiveSilencesWithData tests retrieving silences
// Note: This test uses UTC times to avoid SQLite timezone comparison issues
func TestGetActiveSilencesWithData(t *testing.T) {
	server, db := setupTestServer(t)

	// Create host
	host := models.Host{Name: "test-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}
	host.ID = hostID

	// Create silences (pattern-based, no host constraint)
	// Use UTC time to match SQLite's datetime('now')
	now := time.Now().UTC()
	silences := []*models.NotificationSilence{
		{
			ContainerPattern: "web-*",
			SilencedUntil:    now.Add(1 * time.Hour),
			Reason:           "Test 1",
		},
		{
			ContainerID:   "abc123",
			SilencedUntil: now.Add(2 * time.Hour),
			Reason:        "Test 2",
		},
		{
			// Expired - should not appear
			ContainerPattern: "old-*",
			SilencedUntil:    now.Add(-1 * time.Hour),
			Reason:           "Expired",
		},
	}

	for _, s := range silences {
		if err := db.SaveNotificationSilence(s); err != nil {
			t.Fatalf("SaveNotificationSilence failed: %v", err)
		}
	}

	// Get active silences via API
	req := httptest.NewRequest("GET", "/api/notifications/silences", nil)
	rec := httptest.NewRecorder()

	server.handleGetNotificationSilences(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var retrieved []models.NotificationSilence
	if err := json.NewDecoder(rec.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v. Body: %s", err, rec.Body.String())
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 active silences, got %d", len(retrieved))
		for i, s := range retrieved {
			t.Logf("Retrieved[%d]: ID=%d, Reason=%s", i, s.ID, s.Reason)
		}
	}

	// Verify expired silence is not included
	for _, s := range retrieved {
		if s.ContainerPattern == "old-*" {
			t.Error("Expired silence should not be in active list")
		}
	}
}

// TestCreateSilenceValidation tests validation of silence creation
func TestCreateSilenceValidation(t *testing.T) {
	server, db := setupTestServer(t)

	// Create a host for the host_id test
	host := models.Host{Name: "validation-host", Address: "unix:///", Enabled: true}
	hostID, err := db.AddHost(host)
	if err != nil {
		t.Fatalf("Failed to add host: %v", err)
	}

	testCases := []struct {
		name           string
		body           map[string]interface{}
		expectStatus   int
		errorContains  string
	}{
		{
			name: "Missing all identifiers",
			body: map[string]interface{}{
				"silenced_until": "2026-11-04T14:06",
				"reason":         "Test",
			},
			expectStatus:  http.StatusBadRequest,
			errorContains: "must specify",
		},
		{
			name: "Valid with host_id",
			body: map[string]interface{}{
				"host_id":        hostID,
				"silenced_until": "2026-11-04T14:06",
				"reason":         "Test",
			},
			expectStatus: http.StatusCreated,
		},
		{
			name: "Valid with container_pattern",
			body: map[string]interface{}{
				"container_pattern": "web-*",
				"silenced_until":    "2026-11-04T14:06",
				"reason":            "Test",
			},
			expectStatus: http.StatusCreated,
		},
		{
			name: "Valid with host_pattern",
			body: map[string]interface{}{
				"host_pattern":   "prod-*",
				"silenced_until": "2026-11-04T14:06",
				"reason":         "Test",
			},
			expectStatus: http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/api/notifications/silences", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.handleCreateNotificationSilence(rec, req)

			if rec.Code != tc.expectStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tc.expectStatus, rec.Code, rec.Body.String())
			}

			if tc.errorContains != "" {
				var errResp map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}

				if !contains(errResp["error"], tc.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.errorContains, errResp["error"])
				}
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
